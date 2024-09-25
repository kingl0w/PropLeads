package county

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

type NewHanoverScraper struct{}

// Define the timeout duration as a constant
const elementWaitTimeout = 30 * time.Second

// NewNewHanoverScraper creates a new scraper instance
func NewNewHanoverScraper() *NewHanoverScraper {
    return &NewHanoverScraper{}
}

// Scrape method to scrape multiple PIDs
func (nhs *NewHanoverScraper) Scrape(pids []string) ([]Property, error) {
    opts := append(chromedp.DefaultExecAllocatorOptions[:],
        chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36"),
        chromedp.Flag("ignore-certificate-errors", true),
        chromedp.Flag("disable-web-security", true),
        chromedp.NoSandbox,
        chromedp.Flag("headless", true),
        chromedp.Flag("enable-automation", false),
        chromedp.Flag("disable-blink-features", "AutomationControlled"),
        chromedp.WindowSize(1920, 1080),
    )

    // Create an allocator context for the browser
    allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
    defer cancelAlloc()

    // Create a browser context with custom logger
    browserCtx, cancelBrowser := chromedp.NewContext(allocCtx, chromedp.WithLogf(customLogger))
    defer cancelBrowser()

    var properties []Property
    var mu sync.Mutex
    var wg sync.WaitGroup

    // Limit the number of concurrent goroutines
    concurrencyLimit := 5 // Adjust the number as needed
    sem := make(chan struct{}, concurrencyLimit)

    for _, pid := range pids {
        wg.Add(1)
        sem <- struct{}{}

        go func(pid string) {
            defer wg.Done()
            defer func() { <-sem }()

            // Create a new context for this tab with custom logger
            tabCtx, cancelTab := chromedp.NewContext(browserCtx, chromedp.WithLogf(customLogger))
            defer cancelTab()

            // Set up JavaScript console logging for this tab
            chromedp.ListenTarget(tabCtx, func(ev interface{}) {
                if ev, ok := ev.(*runtime.EventConsoleAPICalled); ok {
                    for _, arg := range ev.Args {
                        log.Printf("Console log: %v", arg)
                    }
                }
            })

            var property Property
            maxAttempts := 3
            for attempt := 1; attempt <= maxAttempts; attempt++ {
                log.Printf("Attempting to scrape PID %s (Attempt %d)", pid, attempt)

                err := nhs.scrapePID(tabCtx, pid, &property)
                if err == nil {
                    log.Printf("Successfully scraped PID %s on attempt %d", pid, attempt)
                    mu.Lock()
                    properties = append(properties, property)
                    mu.Unlock()
                    break
                }

                log.Printf("Error scraping PID %s on attempt %d: %v", pid, attempt, err)
                if attempt < maxAttempts {
                    delay := time.Duration(2000+rand.Intn(3000)) * time.Millisecond
                    log.Printf("Retrying PID %s after %v", pid, delay)
                    time.Sleep(delay)
                } else {
                    log.Printf("Failed to scrape PID %s after %d attempts: %v", pid, maxAttempts, err)
                }
            }
        }(pid)
    }

    wg.Wait()
    return properties, nil
}

// customLogger filters out unwanted log messages
func customLogger(format string, args ...interface{}) {
    if !strings.Contains(format, "could not unmarshal event") {
        log.Printf(format, args...)
    }
}

// scrapePID scrapes data for a single PID
func (nhs *NewHanoverScraper) scrapePID(ctx context.Context, pid string, property *Property) error {
    pid = strings.TrimSpace(pid) // Trim whitespace from PID

    property.ALPHA = pid
    property.PIN = pid // Set the PIN field to PID
    property.COUNTY = "New Hanover"

    log.Printf("Starting scrape for PID %s", pid)

    // Create a new context with a timeout for each scrape attempt
    timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
    defer cancel()

    err := chromedp.Run(timeoutCtx,
        chromedp.Navigate("https://etax.nhcgov.com/pt/search/commonsearch.aspx?mode=parid"),
        chromedp.ActionFunc(func(ctx context.Context) error {
            log.Printf("Navigated to search page for PID %s", pid)
            return nil
        }),
        nhs.handlePossibleDisclaimer(),
        nhs.searchAndClickResult(pid),
        nhs.scrapeMainPage(property),
        nhs.scrapeSalesPage(property),
        nhs.scrapeCommercialPage(property),
        nhs.scrapeValuesPage(property),
    )

    if err != nil {
        return fmt.Errorf("error scraping PID %s: %v", pid, err)
    }

    log.Printf("Successfully scraped PID %s", pid)
    return nil
}

// handlePossibleDisclaimer handles the disclaimer popup
func (nhs *NewHanoverScraper) handlePossibleDisclaimer() chromedp.ActionFunc {
    return func(ctx context.Context) error {
        log.Println("Checking for disclaimer")

        var visible bool
        err := chromedp.Evaluate(`document.querySelector("#btAgree") !== null`, &visible).Do(ctx)
        if err != nil {
            return fmt.Errorf("error checking for disclaimer: %w", err)
        }

        if visible {
            log.Println("Disclaimer found; clicking agree")
            return chromedp.Run(ctx,
                chromedp.Click("#btAgree", chromedp.ByID),
                chromedp.Sleep(1*time.Second),
                chromedp.WaitNotPresent("#btAgree", chromedp.ByID),
            )
        }
        log.Println("No disclaimer present")
        return nil
    }
}

// searchAndClickResult searches for the PID and clicks the result
func (nhs *NewHanoverScraper) searchAndClickResult(pid string) chromedp.ActionFunc {
    return func(ctx context.Context) error {
        log.Printf("Entering PID %s into search form", pid)

        err := chromedp.Run(ctx,
            chromedp.WaitVisible("#inpParid", chromedp.ByID),
            chromedp.Clear("#inpParid", chromedp.ByID),
            chromedp.SendKeys("#inpParid", pid, chromedp.ByID),
            chromedp.SendKeys("#inpParid", kb.Enter),
            waitVisibleWithTimeout(".SearchResults", chromedp.ByQuery, elementWaitTimeout),
            chromedp.Click(".SearchResults", chromedp.ByQuery),
            waitVisibleWithTimeout("#datalet_div_3", chromedp.ByID, elementWaitTimeout),
        )
        if err != nil {
            return fmt.Errorf("error searching and clicking result: %w", err)
        }

        log.Printf("Clicked search result for PID %s", pid)
        return nil
    }
}

// scrapeMainPage scrapes data from the main page
func (nhs *NewHanoverScraper) scrapeMainPage(property *Property) chromedp.ActionFunc {
    return func(ctx context.Context) error {
        log.Printf("Scraping main page for PID %s", property.ALPHA)

        var owner, address, propertyCity, ownerCity, ownerState, ownerZip, acres, zone string

        err := chromedp.Run(ctx,
            waitVisibleWithTimeout(`//*[@id="datalet_header_row"]/td/table/tbody/tr[3]/td[1]`, chromedp.BySearch, elementWaitTimeout),
            chromedp.Text(`//*[@id="datalet_header_row"]/td/table/tbody/tr[3]/td[1]`, &owner, chromedp.BySearch),
            chromedp.Text(`//*[@id="Parcel"]/tbody/tr[2]/td[2]`, &address, chromedp.BySearch),
            chromedp.Text(`//*[@id="Parcel"]/tbody/tr[4]/td[2]`, &propertyCity, chromedp.BySearch),
            chromedp.Text(`//*[@id="Owners (On January1st)"]/tbody/tr[2]/td[2]`, &ownerCity, chromedp.BySearch),
            chromedp.Text(`//*[@id="Owners (On January1st)"]/tbody/tr[3]/td[2]`, &ownerState, chromedp.BySearch),
            chromedp.Text(`//*[@id="Owners (On January1st)"]/tbody/tr[4]/td[2]`, &ownerZip, chromedp.BySearch),
            chromedp.Text(`//*[@id="Parcel"]/tbody/tr[10]/td[2]`, &acres, chromedp.BySearch),
            chromedp.Text(`//*[@id="Parcel"]/tbody/tr[11]/td[2]`, &zone, chromedp.BySearch),
        )
        if err != nil {
            return fmt.Errorf("error scraping main page: %w", err)
        }

        log.Printf("Extracted Data: Owner=%s, Address=%s, Property City=%s, Owner City=%s, Owner State=%s, Owner Zip=%s, Acres=%s, Zone=%s",
            owner, address, propertyCity, ownerCity, ownerState, ownerZip, acres, zone)

        property.NAME = strings.TrimSpace(owner)
        property.PROPERTY_ADDRESS = strings.TrimSpace(address)
        property.PROPERTY_CITY = strings.TrimSpace(propertyCity)
        property.PROPERTY_STATE = "NC" // Set the Property State to "NC"

        property.OWNER_ADDRESS = "" // Set Owner Address as blank
        property.OWNER_CITY = strings.TrimSpace(ownerCity)
        property.OWNER_STATE = strings.TrimSpace(ownerState)
        property.OWNER_ZIP = strings.TrimSpace(ownerZip)
        property.ZONE = strings.TrimSpace(zone)
        property.ACRES = parseFloat(acres)

        log.Printf("Successfully scraped main page for PID %s", property.ALPHA)

        return nil
    }
}

// scrapeSalesPage scrapes data from the sales page
func (nhs *NewHanoverScraper) scrapeSalesPage(property *Property) chromedp.ActionFunc {
    return func(ctx context.Context) error {
        log.Printf("Scraping sales page for PID %s", property.ALPHA)

        var saleDate, salePrice string

        err := chromedp.Run(ctx,
            chromedp.Click(`//*[@id="sidemenu"]/ul/li[2]/a/span`, chromedp.NodeVisible, chromedp.BySearch),
            waitVisibleWithTimeout(`//*[@id="Sale Details"]`, chromedp.BySearch, elementWaitTimeout),
            chromedp.Text(`//*[@id="Sale Details"]/tbody/tr[1]/td[2]`, &saleDate, chromedp.BySearch),
            chromedp.Text(`//*[@id="Sale Details"]/tbody/tr[3]/td[2]`, &salePrice, chromedp.BySearch),
        )
        if err != nil {
            log.Printf("Sales data not found for PID %s: %v", property.ALPHA, err)
            // Continue without sales data
            return nil
        }

        log.Printf("Extracted Sales Data: Sale Date=%s, Sale Price=%s", saleDate, salePrice)

        property.SALE_DATE = strings.TrimSpace(saleDate)
        property.SALE_PRICE = parseFloat(salePrice)

        log.Printf("Successfully scraped sales page for PID %s", property.ALPHA)
        return nil
    }
}

// scrapeCommercialPage scrapes data from the commercial page
func (nhs *NewHanoverScraper) scrapeCommercialPage(property *Property) chromedp.ActionFunc {
    return func(ctx context.Context) error {
        log.Printf("Scraping commercial page for PID %s", property.ALPHA)

        var yearBuilt, sqft string

        err := chromedp.Run(ctx,
            chromedp.Click(`//*[@id="sidemenu"]/ul/li[4]/a/span`, chromedp.NodeVisible, chromedp.BySearch),
            waitVisibleWithTimeout(`//*[@id="Commercial"]`, chromedp.BySearch, elementWaitTimeout),
            chromedp.Text(`//*[@id="Commercial"]/tbody/tr[6]/td[2]`, &yearBuilt, chromedp.BySearch),
            chromedp.Text(`//*[@id="Commercial"]/tbody/tr[12]/td[2]`, &sqft, chromedp.BySearch),
        )
        if err != nil {
            log.Printf("Commercial data not found for PID %s: %v", property.ALPHA, err)
            // Continue without commercial data
            return nil
        }

        log.Printf("Extracted Commercial Data: Year Built=%s, SQFT=%s", yearBuilt, sqft)

        property.YEAR_BUILT = strings.TrimSpace(yearBuilt)
        property.SQFT = parseFloat(sqft)

        log.Printf("Successfully scraped commercial page for PID %s", property.ALPHA)
        return nil
    }
}

// scrapeValuesPage scrapes data from the values page
func (nhs *NewHanoverScraper) scrapeValuesPage(property *Property) chromedp.ActionFunc {
    return func(ctx context.Context) error {
        log.Printf("Scraping values page for PID %s", property.ALPHA)

        var appraised string

        err := chromedp.Run(ctx,
            chromedp.Click(`//*[@id="sidemenu"]/ul/li[8]/a/span`, chromedp.NodeVisible, chromedp.BySearch),
            waitVisibleWithTimeout(`//*[@id="Values"]`, chromedp.BySearch, elementWaitTimeout),
            chromedp.Text(`//*[@id="Values"]/tbody/tr[4]/td[2]`, &appraised, chromedp.BySearch),
        )
        if err != nil {
            log.Printf("Values data not found for PID %s: %v", property.ALPHA, err)
            return nil
        }

        log.Printf("Extracted Values Data: Appraised=%s", appraised)

        property.APPRAISED = parseFloat(appraised)

        log.Printf("Successfully scraped values page for PID %s", property.ALPHA)
        return nil
    }
}

// parseFloat safely parses a string to float64
func parseFloat(s string) float64 {
    // Remove dollar sign and commas
    s = strings.ReplaceAll(s, "$", "")
    s = strings.ReplaceAll(s, ",", "")
    s = strings.TrimSpace(s)

    // Parse the string to float64
    f, err := strconv.ParseFloat(s, 64)
    if err != nil {
        // Handle error or return a default value
        return 0
    }
    return f
}

// waitVisibleWithTimeout waits for an element to be visible with a timeout
func waitVisibleWithTimeout(selector string, selType chromedp.QueryOption, timeout time.Duration) chromedp.ActionFunc {
    return func(ctx context.Context) error {
        ctx, cancel := context.WithTimeout(ctx, timeout)
        defer cancel()
        return chromedp.WaitVisible(selector, selType).Do(ctx)
    }
}
