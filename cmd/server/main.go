package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/kingl0w/PropLeads/internal/auth"
	csv "github.com/kingl0w/PropLeads/internal/csvutil"
	"github.com/kingl0w/PropLeads/internal/county"
	"github.com/kingl0w/PropLeads/internal/database"
	"github.com/kingl0w/PropLeads/internal/dataprocessing"
	"github.com/kingl0w/PropLeads/internal/sos"
)

type ScrapeRequest struct {
	County string   `json:"county"`
	PIDs   []string `json:"pids"`
}

type JobStatus struct {
	JobID    string         `json:"jobId"`
	Status   string         `json:"status"`
	Progress int            `json:"progress"`
	Message  string         `json:"message"`
	Steps    []ProgressStep `json:"steps,omitempty"`
	Results  *JobResults    `json:"results,omitempty"`
}

type ProgressStep struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type JobResults struct {
	PropertiesScraped     int `json:"propertiesScraped"`
	BusinessesIdentified  int `json:"businessesIdentified"`
	IndividualsExtracted  int `json:"individualsExtracted"`
}

type JobManager struct {
	mu   sync.RWMutex
	jobs map[string]*JobStatus
}

var jobManager = &JobManager{
	jobs: make(map[string]*JobStatus),
}

func (jm *JobManager) CreateJob(jobID string) *JobStatus {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	status := &JobStatus{
		JobID:    jobID,
		Status:   "pending",
		Progress: 0,
		Message:  "Job created",
		Steps: []ProgressStep{
			{ID: "1", Label: "Upload PIDs", Status: "completed", Message: "PIDs received"},
			{ID: "2", Label: "County property search", Status: "pending"},
			{ID: "3", Label: "SOS business lookup", Status: "pending"},
			{ID: "4", Label: "Data processing", Status: "pending"},
			{ID: "5", Label: "Generating exports", Status: "pending"},
		},
	}
	jm.jobs[jobID] = status
	return status
}

func (jm *JobManager) GetJob(jobID string) (*JobStatus, bool) {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	job, exists := jm.jobs[jobID]
	return job, exists
}

func (jm *JobManager) UpdateJob(jobID string, updater func(*JobStatus)) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if job, exists := jm.jobs[jobID]; exists {
		updater(job)
	}
}

func handleSignup(w http.ResponseWriter, r *http.Request) {
	var req auth.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := auth.CreateUser(req.Email, req.Username, req.Password)
	if err != nil {
		if err == auth.ErrUserExists {
			http.Error(w, "User with this email already exists", http.StatusConflict)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	token, err := auth.GenerateToken(user)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(auth.AuthResponse{
		Token: token,
		User:  *user,
	})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := auth.AuthenticateUser(req.Email, req.Password)
	if err != nil {
		if err == auth.ErrInvalidCredentials {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		} else if err == auth.ErrInactiveAccount {
			http.Error(w, "Account is inactive", http.StatusForbidden)
		} else {
			http.Error(w, "Authentication failed", http.StatusInternalServerError)
		}
		return
	}

	token, err := auth.GenerateToken(user)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(auth.AuthResponse{
		Token: token,
		User:  *user,
	})
}

func handleScrapeStart(w http.ResponseWriter, r *http.Request) {
	var req ScrapeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jobID := uuid.New().String()
	jobManager.CreateJob(jobID)

	go runScrapeJob(jobID, req.County, req.PIDs)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"jobId": jobID})
}

func handleJobStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["jobId"]

	job, exists := jobManager.GetJob(jobID)
	if !exists {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["jobId"]
	filename := vars["filename"]

	//prevent directory traversal
	if filepath.Base(filename) != filename {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	var actualFilename string
	switch filename {
	case "parcel_results.csv":
		actualFilename = fmt.Sprintf("parcel_results_%s.csv", jobID)
	case "sos_results.csv":
		actualFilename = fmt.Sprintf("sos_results_%s.csv", jobID)
	case "unified_results.csv":
		actualFilename = fmt.Sprintf("unified_results_%s.csv", jobID)
	case "names.csv":
		actualFilename = fmt.Sprintf("names_%s.csv", jobID)
	default:
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	filePath := filepath.Join("data", "output", actualFilename)

	if _, err := filepath.Abs(filePath); err != nil {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	http.ServeFile(w, r, filePath)
}

func runScrapeJob(jobID, countyName string, pids []string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Job %s panicked: %v", jobID, r)
			jobManager.UpdateJob(jobID, func(js *JobStatus) {
				js.Status = "failed"
				js.Message = fmt.Sprintf("Error: %v", r)
			})
		}
	}()

	jobManager.UpdateJob(jobID, func(js *JobStatus) {
		js.Status = "processing"
		js.Progress = 5
		js.Message = "Starting property search..."
		js.Steps[1].Status = "processing"
	})

	var scraper county.Scraper
	switch countyName {
	case "newhanover":
		scraper = county.NewNewHanoverScraper()
	case "brunswick":
		scraper = county.NewBrunswickScraper()
	case "pender":
		scraper = county.NewPenderScraper()
	default:
		jobManager.UpdateJob(jobID, func(js *JobStatus) {
			js.Status = "failed"
			js.Message = "Unknown county: " + countyName
		})
		return
	}

	properties, err := scraper.Scrape(pids)
	if err != nil {
		jobManager.UpdateJob(jobID, func(js *JobStatus) {
			js.Status = "failed"
			js.Message = fmt.Sprintf("County scrape failed: %v", err)
		})
		return
	}

	parcelFile := filepath.Join("data", "output", fmt.Sprintf("parcel_results_%s.csv", jobID))
	csv.WriteParcelResults(parcelFile, properties)

	jobManager.UpdateJob(jobID, func(js *JobStatus) {
		js.Progress = 30
		js.Steps[1].Status = "completed"
		js.Steps[1].Message = fmt.Sprintf("%d properties found", len(properties))
		js.Steps[2].Status = "processing"
		js.Message = "Searching for business owners..."
	})

	parcels, _ := csv.ReadParcelResults(parcelFile)
	uniqueBusinesses := make(map[string]bool)
	for _, parcel := range parcels {
		ownerName, ok := parcel["Owner"]
		if ok && csv.IsBusinessName(ownerName) {
			uniqueBusinesses[ownerName] = true
		}
	}

	businessNames := make([]string, 0, len(uniqueBusinesses))
	for name := range uniqueBusinesses {
		businessNames = append(businessNames, name)
	}

	workers := 5
	if len(businessNames) >= 100 {
		workers = 8
	}

	var businessInfos []sos.BusinessInfo
	var businessMu sync.Mutex
	processedCount := 0
	jobs := make(chan string, len(businessNames))
	var wg sync.WaitGroup

	for w := 1; w <= workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for businessName := range jobs {
				info, err := sos.LookupBusiness(businessName)

				if err != nil {
					log.Printf("SOS lookup failed for '%s': %v", businessName, err)
				} else if len(info.CompanyOfficials) > 0 {
					firstOfficial := info.CompanyOfficials[0]
					if firstOfficial.Title != "Result" && firstOfficial.Title != "Error" {
						businessMu.Lock()
						businessInfos = append(businessInfos, info)
						businessMu.Unlock()
					} else {
						log.Printf("SOS lookup returned no match for '%s': %s", businessName, firstOfficial.Name)
					}
				}

				businessMu.Lock()
				processedCount++
				completed := processedCount
				businessMu.Unlock()

				progress := 30 + int(float64(completed)/float64(len(businessNames))*40)
				jobManager.UpdateJob(jobID, func(js *JobStatus) {
					js.Progress = progress
					js.Steps[2].Message = fmt.Sprintf("%d/%d businesses processed", completed, len(businessNames))
				})
			}
		}(w)
	}

	for _, name := range businessNames {
		jobs <- name
	}
	close(jobs)

	wg.Wait()

	sosFile := filepath.Join("data", "output", fmt.Sprintf("sos_results_%s.csv", jobID))
	csv.WriteSOSResults(sosFile, businessInfos)

	jobManager.UpdateJob(jobID, func(js *JobStatus) {
		js.Progress = 70
		js.Steps[2].Status = "completed"
		js.Steps[2].Message = fmt.Sprintf("%d businesses processed", len(businessNames))
		js.Steps[3].Status = "processing"
		js.Message = "Processing and merging data..."
	})

	unifiedFile := filepath.Join("data", "output", fmt.Sprintf("unified_results_%s.csv", jobID))
	namesFile := filepath.Join("data", "output", fmt.Sprintf("names_%s.csv", jobID))

	err = dataprocessing.ProcessData(parcelFile, sosFile, unifiedFile, namesFile)
	if err != nil {
		log.Printf("Data processing failed: %v", err)
	}

	jobManager.UpdateJob(jobID, func(js *JobStatus) {
		js.Progress = 90
		js.Steps[3].Status = "completed"
		js.Steps[4].Status = "processing"
		js.Message = "Finalizing exports..."
	})

	time.Sleep(1 * time.Second)

	individualsCount := 0
	if unifiedData, err := csv.ReadParcelResults(unifiedFile); err == nil {
		individualsCount = len(unifiedData)
	}

	jobManager.UpdateJob(jobID, func(js *JobStatus) {
		js.Status = "completed"
		js.Progress = 100
		js.Message = "Scraping complete!"
		js.Steps[4].Status = "completed"
		js.Results = &JobResults{
			PropertiesScraped:    len(properties),
			BusinessesIdentified: len(businessNames),
			IndividualsExtracted: individualsCount,
		}
	})
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	router := mux.NewRouter()

	router.HandleFunc("/api/auth/signup", handleSignup).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/auth/login", handleLogin).Methods("POST", "OPTIONS")

	router.HandleFunc("/api/scrape", auth.AuthMiddleware(handleScrapeStart)).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/scrape/{jobId}/status", auth.AuthMiddleware(handleJobStatus)).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/scrape/{jobId}/download/{filename}", auth.AuthMiddleware(handleDownload)).Methods("GET", "OPTIONS")

	handler := enableCORS(router)

	port := "8080"
	fmt.Printf("🚀 PropLeads API Server starting on port %s\n", port)
	fmt.Printf("🔐 Authentication enabled - users must signup/login\n")
	fmt.Printf("📊 Frontend should connect to: http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
