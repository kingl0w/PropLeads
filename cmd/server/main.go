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

// API Types matching frontend
type ScrapeRequest struct {
	County string   `json:"county"`
	PIDs   []string `json:"pids"`
}

type JobStatus struct {
	JobID    string         `json:"jobId"`
	Status   string         `json:"status"` // pending, processing, completed, failed
	Progress int            `json:"progress"`
	Message  string         `json:"message"`
	Steps    []ProgressStep `json:"steps,omitempty"`
	Results  *JobResults    `json:"results,omitempty"`
}

type ProgressStep struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Status  string `json:"status"` // pending, processing, completed, failed
	Message string `json:"message,omitempty"`
}

type JobResults struct {
	PropertiesScraped     int `json:"propertiesScraped"`
	BusinessesIdentified  int `json:"businessesIdentified"`
	IndividualsExtracted  int `json:"individualsExtracted"`
}

// Job manager
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

// Auth Handlers
func handleSignup(w http.ResponseWriter, r *http.Request) {
	var req auth.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create user
	user, err := auth.CreateUser(req.Email, req.Username, req.Password)
	if err != nil {
		if err == auth.ErrUserExists {
			http.Error(w, "User with this email already exists", http.StatusConflict)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	// Generate token
	token, err := auth.GenerateToken(user)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Return token and user
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

	// Authenticate user
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

	// Generate token
	token, err := auth.GenerateToken(user)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Return token and user
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(auth.AuthResponse{
		Token: token,
		User:  *user,
	})
}

// Scraping Handlers (Protected)
func handleScrapeStart(w http.ResponseWriter, r *http.Request) {
	var req ScrapeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create job ID
	jobID := uuid.New().String()
	jobManager.CreateJob(jobID)

	// Start scraping in background
	go runScrapeJob(jobID, req.County, req.PIDs)

	// Return job ID
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

	// Update status to processing
	jobManager.UpdateJob(jobID, func(js *JobStatus) {
		js.Status = "processing"
		js.Progress = 5
		js.Message = "Starting property search..."
		js.Steps[1].Status = "processing"
	})

	// Step 2: County property search
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

	// Write parcel results
	parcelFile := filepath.Join("data", "output", fmt.Sprintf("parcel_results_%s.csv", jobID))
	csv.WriteParcelResults(parcelFile, properties)

	jobManager.UpdateJob(jobID, func(js *JobStatus) {
		js.Progress = 30
		js.Steps[1].Status = "completed"
		js.Steps[1].Message = fmt.Sprintf("%d properties found", len(properties))
		js.Steps[2].Status = "processing"
		js.Message = "Searching for business owners..."
	})

	// Step 3: SOS business lookup
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

	// Determine workers (auto-scale based on business count)
	workers := 5
	if len(businessNames) >= 100 {
		workers = 8
	}

	// Process businesses concurrently
	var businessInfos []sos.BusinessInfo
	jobs := make(chan string, len(businessNames))
	results := make(chan sos.BusinessInfo, len(businessNames))

	// Start workers
	for w := 1; w <= workers; w++ {
		go func(id int) {
			for businessName := range jobs {
				info, err := sos.LookupBusiness(businessName)
				if err != nil {
					info = sos.BusinessInfo{
						BusinessName:     businessName,
						CompanyOfficials: []sos.Official{{Title: "Result", Name: "Error"}},
					}
				}

				// Update progress
				completed := len(businessInfos) + 1
				progress := 30 + int(float64(completed)/float64(len(businessNames))*40)
				jobManager.UpdateJob(jobID, func(js *JobStatus) {
					js.Progress = progress
					js.Steps[2].Message = fmt.Sprintf("%d/%d businesses processed", completed, len(businessNames))
				})

				results <- info
			}
		}(w)
	}

	// Send jobs
	for _, name := range businessNames {
		jobs <- name
	}
	close(jobs)

	// Collect results
	for i := 0; i < len(businessNames); i++ {
		businessInfos = append(businessInfos, <-results)
	}

	sosFile := filepath.Join("data", "output", fmt.Sprintf("sos_results_%s.csv", jobID))
	csv.WriteSOSResults(sosFile, businessInfos)

	jobManager.UpdateJob(jobID, func(js *JobStatus) {
		js.Progress = 70
		js.Steps[2].Status = "completed"
		js.Steps[2].Message = fmt.Sprintf("%d businesses processed", len(businessNames))
		js.Steps[3].Status = "processing"
		js.Message = "Processing and merging data..."
	})

	// Step 4: Data processing
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

	// Step 5: Complete
	time.Sleep(1 * time.Second) // Small delay for UX

	// Count individuals from unified results
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Initialize database
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	router := mux.NewRouter()

	// Public routes (no auth required)
	router.HandleFunc("/api/auth/signup", handleSignup).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/auth/login", handleLogin).Methods("POST", "OPTIONS")

	// Protected routes (auth required)
	router.HandleFunc("/api/scrape", auth.AuthMiddleware(handleScrapeStart)).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/scrape/{jobId}/status", auth.AuthMiddleware(handleJobStatus)).Methods("GET", "OPTIONS")

	// Wrap with CORS middleware
	handler := enableCORS(router)

	port := "8080"
	fmt.Printf("🚀 PropLeads API Server starting on port %s\n", port)
	fmt.Printf("🔐 Authentication enabled - users must signup/login\n")
	fmt.Printf("📊 Frontend should connect to: http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
