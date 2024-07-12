package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/kingl0w/PropLeads/internal/county"
	"github.com/kingl0w/PropLeads/internal/csv"
	"github.com/kingl0w/PropLeads/internal/sos"
)

    

func main() {
    countyName := flag.String("county", "", "Name of the county to scrape")
    sosOnly := flag.Bool("sos-only", false, "Run only the SOS scrape")
    flag.Parse()

    if *sosOnly {
        runSOSScrape()
    } else {
        runCountyScrape(*countyName)
    }
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

    var businessInfos []sos.BusinessInfo
    var timedOutBusinesses []string

    for businessName := range uniqueBusinesses {
        fmt.Printf("Looking up business: %s\n", businessName)
        info, err := sos.LookupBusiness(businessName)
        if err != nil {
            log.Printf("Error looking up business %s: %v\n", businessName, err)
            continue
        }

        if len(info.CompanyOfficials) > 0 && info.CompanyOfficials[0].Name == "Timeout" {
            timedOutBusinesses = append(timedOutBusinesses, businessName)
        }

        businessInfos = append(businessInfos, info)

        if len(info.CompanyOfficials) == 0 {
            log.Printf("No officials found for business %s\n", businessName)
        } else {
            log.Printf("Found %d officials for %s\n", len(info.CompanyOfficials), businessName)
            for i, official := range info.CompanyOfficials {
                log.Printf("  Official %d: %s - %s", i+1, official.Title, official.Name)
            }
        }
    }

    // Retry timed out businesses
    if len(timedOutBusinesses) > 0 {
        log.Printf("Retrying %d timed out businesses", len(timedOutBusinesses))
        retryResults := sos.RetryTimedOutBusinesses(timedOutBusinesses)

        // Replace timed out results with successful retries
        for _, retryInfo := range retryResults {
            for i, info := range businessInfos {
                if info.BusinessName == retryInfo.BusinessName {
                    businessInfos[i] = retryInfo
                    log.Printf("Retry successful for %s", retryInfo.BusinessName)
                    break
                }
            }
        }
    }

    err = csv.WriteSOSResults(sosResultsFile, businessInfos)
    if err != nil {
        log.Fatalf("Failed to write SOS results: %v", err)
    }
    fmt.Printf("SOS results written to %s\n", sosResultsFile)
}