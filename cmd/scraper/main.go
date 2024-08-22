package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"path/filepath"
	"sync"
	"time"

	"github.com/kingl0w/PropLeads/internal/county"
	csv "github.com/kingl0w/PropLeads/internal/csvutil"
	"github.com/kingl0w/PropLeads/internal/dataprocessing"
	"github.com/kingl0w/PropLeads/internal/reconciliation"
	"github.com/kingl0w/PropLeads/internal/sos"
)

    

func main() {
    countyName := flag.String("county", "", "Name of the county to scrape")
    sosOnly := flag.Bool("sos-only", false, "Run only the SOS scrape")
    processOnly := flag.Bool("process-only", false, "Run only the data processing step")
    reconcileOnly := flag.Bool("reconcile-only", false, "Run only the data reconciliation step")

    flag.Parse()

    if *reconcileOnly {
        runReconciliation()
    } else if *processOnly {
        runDataProcessing()
    } else if *sosOnly {
        runSOSScrape()
    } else {
        runCountyScrape(*countyName)
    }
}

func runReconciliation() {
    unifiedResultsPath := filepath.Join("data", "output", "unified_results.csv")
    wpSearchPattern := filepath.Join("data", "input", "WP_*.csv")
    outputPath := filepath.Join("data", "output", "final_results.csv")

    wpFiles, err := filepath.Glob(wpSearchPattern)
    if err != nil {
        log.Fatalf("Error finding WP search results: %v", err)
    }
    if len(wpFiles) == 0 {
        log.Fatalf("No WP search results file found matching pattern: %s", wpSearchPattern)
    }
    wpSearchPath := wpFiles[0] // Use the first matching file

    err = reconciliation.ReconcileData(unifiedResultsPath, wpSearchPath, outputPath)
    if err != nil {
        log.Fatalf("Data reconciliation failed: %v", err)
    }

    fmt.Println("Reconciliation complete. Final results file created:", outputPath)
}



func runDataProcessing() {
    parcelResultsFile := filepath.Join("data", "output", "parcel_results.csv")
    sosResultsFile := filepath.Join("data", "output", "sos_results.csv")
    unifiedResultsFile := filepath.Join("data", "output", "unified_results.csv")
    namesFile := filepath.Join("data", "output", "names.csv")

    err := dataprocessing.ProcessData(parcelResultsFile, sosResultsFile, unifiedResultsFile, namesFile)
    if err != nil {
        log.Fatalf("Data processing failed: %v", err)
    }

    fmt.Println("Processing complete. Unified results file created:", unifiedResultsFile)
    fmt.Println("Names file created:", namesFile)
}

func runCountyScrape(countyName string) {
    pids, err := csv.ReadPIDs("data/input/pids.csv")
    if err != nil {
        log.Fatalf("Failed to read PIDs: %v", err)
    }

    var scraper county.Scraper
    switch countyName {
    case "pender":
        scraper = county.NewPenderScraper()
    // Add cases for other counties
    default:
        log.Fatalf("Unknown county: %s", countyName)
    }

    properties, err := scraper.Scrape(pids)
    if err != nil {
        log.Fatalf("Failed to scrape: %v", err)
    }

    // Print to console
    for _, info := range properties {
        fmt.Printf("Parcel Information:\n")
        fmt.Printf("  ID: %s\n", info.ALPHA)
        fmt.Printf("  PIN: %s\n", info.PIN)
        fmt.Printf("  Owner: %s\n", info.NAME)
        fmt.Printf("  Property Address: %s\n", info.PROPERTY_ADDRESS)
        fmt.Printf("  Owner Address: %s, %s, %s %s\n", info.ADDR, info.CITY, info.STATE, info.ZIP)
        fmt.Printf("  Acres: %.2f (Calculated: %.2f)\n", info.ACRES, info.CALCACRES)
        fmt.Printf("  Zone: %s\n", info.ZONE)
        fmt.Printf("  Tax Codes: %s\n", info.TAX_CODES)
        if info.SALE_PRICE > 0 {
            fmt.Printf("  Sale Price: $%.2f\n", info.SALE_PRICE)
        } else {
            fmt.Printf("  Sale Price: Not available\n")
        }
        fmt.Println()
    }

    // Write results to CSV
    outputFilename := "data/output/parcel_results.csv"
    err = csv.WriteParcelResults(outputFilename, properties)
    if err != nil {
        fmt.Printf("Error writing to CSV: %v\n", err)
    } else {
        fmt.Printf("Results written to %s\n", outputFilename)
    }

    // Run SOS scrape after county scrape
    runSOSScrape()
}

func runSOSScrape() {
    sos.VerboseLogging = false

    parcelResultsFile := filepath.Join("data", "output", "parcel_results.csv")
    sosResultsFile := filepath.Join("data", "output", "sos_results.csv")

    // Read parcel results
    parcels, err := csv.ReadParcelResults(parcelResultsFile)
    if err != nil {
        log.Fatalf("Failed to read parcel results: %v", err)
    }

    // Extract unique business names
    uniqueBusinesses := make(map[string]bool)
    for _, parcel := range parcels {
        ownerName := parcel["Owner"]
        if csv.IsBusinessName(ownerName) {
            uniqueBusinesses[ownerName] = true
        }
    }

    jobs := make(chan string, len(uniqueBusinesses))
    results := make(chan sos.BusinessInfo, len(uniqueBusinesses))

    // Start worker goroutines
    numWorkers := 5
    var wg sync.WaitGroup
    for w := 1; w <= numWorkers; w++ {
        wg.Add(1)
        go worker(w, jobs, results, &wg)
    }

    // Send jobs to the worker pool
    for businessName := range uniqueBusinesses {
        jobs <- businessName
    }
    close(jobs)

    // Start a goroutine to close results channel when all workers are done
    go func() {
        wg.Wait()
        close(results)
    }()

    // Collect results
    var businessInfos []sos.BusinessInfo
    for info := range results {
        businessInfos = append(businessInfos, info)
        if len(info.CompanyOfficials) == 0 {
            log.Printf("No officials found for business %s\n", info.BusinessName)
        } else {
            log.Printf("Found %d officials for %s\n", len(info.CompanyOfficials), info.BusinessName)
            for i, official := range info.CompanyOfficials {
                log.Printf("  Official %d: %s - %s", i+1, official.Title, official.Name)
            }
        }
    }

    err = csv.WriteSOSResults(sosResultsFile, businessInfos)
    if err != nil {
        log.Fatalf("Failed to write SOS results: %v", err)
    }
    fmt.Printf("SOS results written to %s\n", sosResultsFile)
}

func worker(id int, jobs <-chan string, results chan<- sos.BusinessInfo, wg *sync.WaitGroup) {
    defer wg.Done()
    for businessName := range jobs {
        var info sos.BusinessInfo
        var err error
        for attempts := 0; attempts < 3; attempts++ {
            if sos.VerboseLogging {
                fmt.Printf("Worker %d looking up business: %s (Attempt %d)\n", id, businessName, attempts+1)
            }
            info, err = sos.LookupBusiness(businessName)
            if err == nil && len(info.CompanyOfficials) > 0 && info.CompanyOfficials[0].Name != "No match" {
                break
            }
            if attempts < 2 {
                delay := time.Duration(2000 + rand.Intn(3000)) * time.Millisecond
                time.Sleep(delay)
            }
        }
        if err != nil {
            if sos.VerboseLogging {
                log.Printf("Error looking up business %s: %v\n", businessName, err)
            }
            info = sos.BusinessInfo{BusinessName: businessName, CompanyOfficials: []sos.Official{{Title: "Result", Name: "No match"}}}
        }
        results <- info
    }
}