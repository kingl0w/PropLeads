package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

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
    workers := flag.Int("workers", 0, "Number of concurrent workers for SOS scraping (0 = auto-scale, default)")

    flag.Parse()

    if *reconcileOnly {
        runReconciliation()
    } else if *processOnly {
        runDataProcessing()
    } else if *sosOnly {
        runSOSScrape(*workers)
    } else {
        runCountyScrape(*countyName, *workers)
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
    wpNamesFile := filepath.Join("data", "output", "names_for_whitepages.csv")

    err := dataprocessing.ProcessData(parcelResultsFile, sosResultsFile, unifiedResultsFile, namesFile)
    if err != nil {
        log.Fatalf("Data processing failed: %v", err)
    }

    fmt.Println("Processing complete. Unified results file created:", unifiedResultsFile)
    fmt.Println("Names file created:", namesFile)
    fmt.Println("WhitePages upload file created:", wpNamesFile)
    fmt.Println("\nNext step: Upload", wpNamesFile, "to WhitePages to get contact info")
}

func runCountyScrape(countyName string, workers int) {
    pids, err := csv.ReadPIDs("data/input/pids.csv")
    if err != nil {
        log.Fatalf("Failed to read PIDs: %v", err)
    }

    var scraper county.Scraper
    switch countyName {
    case "pender":
        scraper = county.NewPenderScraper()
    case "newhanover":
        scraper = county.NewNewHanoverScraper()
    case "brunswick":
        scraper = county.NewBrunswickScraper()
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
        fmt.Printf("  Property City: %s\n", info.PROPERTY_CITY)
        fmt.Printf("  Owner Address: %s, %s, %s %s\n", info.OWNER_ADDRESS, info.OWNER_CITY, info.OWNER_STATE, info.OWNER_ZIP)
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
    runSOSScrape(workers)
}

func runSOSScrape(workers int) {
    fmt.Println("\n=== Starting SOS Scrape ===")

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
        ownerName, ok := parcel["Owner"]
        if !ok {
            log.Printf("Warning: 'Owner' field not found in parcel data")
            continue
        }
        if csv.IsBusinessName(ownerName) {
            uniqueBusinesses[ownerName] = true
        }
    }

    fmt.Printf("Found %d unique businesses to look up\n", len(uniqueBusinesses))

    // Convert map to slice for easier processing
    businessNames := make([]string, 0, len(uniqueBusinesses))
    for name := range uniqueBusinesses {
        businessNames = append(businessNames, name)
    }

    // Determine worker count (auto-scale if workers == 0)
    numWorkers := determineWorkers(workers, len(businessNames))
    fmt.Printf("Using %d concurrent workers\n", numWorkers)

    // Process businesses concurrently
    businessInfos := processConcurrent(businessNames, numWorkers)

    // Write results
    err = csv.WriteSOSResults(sosResultsFile, businessInfos)
    if err != nil {
        log.Fatalf("Failed to write SOS results: %v", err)
    }
    fmt.Printf("\nSOS results written to %s\n", sosResultsFile)
}

func determineWorkers(userWorkers int, numBusinesses int) int {
    // If user specified workers, use that
    if userWorkers > 0 {
        return userWorkers
    }

    // Auto-scale based on number of businesses
    switch {
    case numBusinesses < 10:
        return 3 // Small batch: conservative
    case numBusinesses < 50:
        return 5 // Medium batch: default
    case numBusinesses < 100:
        return 5 // Still default, stay safe
    case numBusinesses < 200:
        return 8 // Large batch: scale up
    default:
        return 10 // Very large batch: max workers
    }
}

func processConcurrent(businessNames []string, workers int) []sos.BusinessInfo {
    // Create channels
    jobs := make(chan string, len(businessNames))
    results := make(chan sos.BusinessInfo, len(businessNames))

    // Start workers
    for w := 1; w <= workers; w++ {
        go worker(w, jobs, results)
    }

    // Send jobs
    for _, name := range businessNames {
        jobs <- name
    }
    close(jobs)

    // Collect results
    var businessInfos []sos.BusinessInfo
    for i := 0; i < len(businessNames); i++ {
        info := <-results
        businessInfos = append(businessInfos, info)
    }

    return businessInfos
}

func worker(id int, jobs <-chan string, results chan<- sos.BusinessInfo) {
    for businessName := range jobs {
        info, err := sos.LookupBusiness(businessName)
        if err != nil {
            log.Printf("[Worker %d] Error looking up %s: %v", id, businessName, err)
            info = sos.BusinessInfo{
                BusinessName:     businessName,
                CompanyOfficials: []sos.Official{{Title: "Result", Name: "Error"}},
            }
        }

        if len(info.CompanyOfficials) > 0 && info.CompanyOfficials[0].Name != "No match" && info.CompanyOfficials[0].Name != "Error" {
            log.Printf("[Worker %d] ✓ Found %d officials for %s", id, len(info.CompanyOfficials), info.BusinessName)
        } else {
            log.Printf("[Worker %d] ✗ No officials found for %s", id, info.BusinessName)
        }

        results <- info
    }
}

