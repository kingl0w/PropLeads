package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/kingl0w/PropLeads/internal/county"
	"github.com/kingl0w/PropLeads/internal/csv"
)

func main() {
    countyName := flag.String("county", "", "Name of the county to scrape")
    flag.Parse()

    pids, err := csv.ReadPIDs("data/input/pids.csv")
    if err != nil {
        log.Fatalf("Failed to read PIDs: %v", err)
    }

    var scraper county.Scraper
    switch *countyName {
    case "pender":
        scraper = county.NewPenderScraper()
    // Add cases for other counties
    default:
        log.Fatalf("Unknown county: %s", *countyName)
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
}