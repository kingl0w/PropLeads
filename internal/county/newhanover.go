package county

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

type NewHanoverScraper struct{}

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

    allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
    defer cancelAlloc()

    var properties []Property

    for _, pid := range pids {
        ctx, cancelCtx := chromedp.NewContext(allocCtx, chromedp.WithLogf(customLogger))
        defer cancelCtx()

        // Set up JavaScript console logging
        chromedp.ListenTarget(ctx, func(ev interface{}) {
            if ev, ok := ev.(*runtime.EventConsoleAPICalled); ok {
                for _, arg := range ev.Args {
                    log.Printf("Console log: %v", arg)
                }
            }
        })

        var property Property
        err := retry(3, 5*time.Second, func() error {
            var err error
            property, err = nhs.scrapePID(ctx, pid)
            return err
        })
        if err != nil {
            log.Printf("Failed to scrape PID %s after retries: %v", pid, err)
            continue
        }
        properties = append(properties, property)

        time.Sleep(5 * time.Second)
    }

    return properties, nil
}

// customLogger filters out unwanted log messages
func customLogger(format string, args ...interface{}) {
    if !strings.Contains(format, "could not unmarshal event") {
        log.Printf(format, args...)
    }
}

// scrapePID scrapes data for a single PID
func (nhs *NewHanoverScraper) scrapePID(ctx context.Context, pid string) (Property, error) {
    pid = strings.TrimSpace(pid) // Trim whitespace from PID

    var property Property
    property.ALPHA = pid
    property.PIN = pid       // Set the PIN field to PID
    property.COUNTY = "New Hanover"

    log.Printf("Starting scrape for PID %s", pid)

    // Added timeout context to the scrapePID function
    timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
    defer cancel()

    err := chromedp.Run(timeoutCtx,
        chromedp.Navigate("https://etax.nhcgov.com/pt/search/commonsearch.aspx?mode=parid"),
        chromedp.ActionFunc(func(ctx context.Context) error {
            log.Printf("Navigated to search page for PID %s", pid)
            return nil
        }),
        nhs.handlePossibleDisclaimer(),
        nhs.searchAndClickResult(pid),
        nhs.scrapeMainPage(&property),
        nhs.scrapeSalesPage(&property),
        nhs.scrapeCommercialPage(&property),
        nhs.scrapeValuesPage(&property),
    )

    if err != nil {
        return Property{}, fmt.Errorf("error scraping PID %s: %v", pid, err)
    }

    log.Printf("Successfully scraped PID %s", pid)
    return property, nil
}

// handlePossibleDisclaimer handles the disclaimer popup
func (nhs *NewHanoverScraper) handlePossibleDisclaimer() chromedp.ActionFunc {
    // ... existing code ...
    return func(ctx context.Context) error {
        log.Println("Checking for disclaimer")

        var visible bool
        err := chromedp.Run(ctx,
            chromedp.Evaluate(`document.querySelector("#btAgree") !== null`, &visible),
        )
        if err != nil {
            return fmt.Errorf("error checking for disclaimer: %w", err)
        }

        if visible {
            log.Println("Disclaimer found; clicking agree")
            return chromedp.Run(ctx,
                chromedp.Click("#btAgree", chromedp.ByID),
                chromedp.WaitNotPresent("#btAgree", chromedp.ByID),
            )
        }
        log.Println("No disclaimer present")
        return nil
    }
}

// searchAndClickResult searches for the PID and clicks the result
func (nhs *NewHanoverScraper) searchAndClickResult(pid string) chromedp.ActionFunc {
    // ... existing code ...
    return func(ctx context.Context) error {
        // Create a context with a timeout of 60 seconds
        timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
        defer cancel()

        log.Printf("Entering PID %s into search form", pid)

        err := chromedp.Run(timeoutCtx,
            chromedp.WaitVisible("#inpParid", chromedp.ByID),
            chromedp.Clear("#inpParid", chromedp.ByID),
            chromedp.SendKeys("#inpParid", pid, chromedp.ByID),
            chromedp.SendKeys("#inpParid", kb.Enter),
            chromedp.WaitVisible(".SearchResults", chromedp.ByQuery),
            chromedp.Click(".SearchResults", chromedp.ByQuery),
            chromedp.WaitVisible("#datalet_div_3", chromedp.ByID),
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

        timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
        defer cancel()

        var owner, address, propertyCity, ownerCity, ownerState, acres, zone string

        err := chromedp.Run(timeoutCtx,
            chromedp.WaitVisible(`//*[@id="datalet_header_row"]/td/table/tbody/tr[3]/td[1]`, chromedp.BySearch),
            chromedp.Text(`//*[@id="datalet_header_row"]/td/table/tbody/tr[3]/td[1]`, &owner, chromedp.BySearch),
            chromedp.Text(`//*[@id="Parcel"]/tbody/tr[2]/td[2]`, &address, chromedp.BySearch),
            chromedp.Text(`//*[@id="Parcel"]/tbody/tr[4]/td[2]`, &propertyCity, chromedp.BySearch),
            chromedp.Text(`//*[@id="Owners (On January1st)"]/tbody/tr[2]/td[2]`, &ownerCity, chromedp.BySearch),
            chromedp.Text(`//*[@id="Owners (On January1st)"]/tbody/tr[3]/td[2]`, &ownerState, chromedp.BySearch),
            chromedp.Text(`//*[@id="Parcel"]/tbody/tr[10]/td[2]`, &acres, chromedp.BySearch),
            chromedp.Text(`//*[@id="Parcel"]/tbody/tr[11]/td[2]`, &zone, chromedp.BySearch),
        )
        if err != nil {
            return fmt.Errorf("error scraping main page: %w", err)
        }

        log.Printf("Extracted Data: Owner=%s, Address=%s, Property City=%s, Owner City=%s, Owner State=%s, Acres=%s, Zone=%s",
            owner, address, propertyCity, ownerCity, ownerState, acres, zone)

        property.NAME = owner
        property.PROPERTY_ADDRESS = address
        property.PROPERTY_CITY = propertyCity
        property.PROPERTY_STATE = "NC"     // Set the Property State to "NC"
        property.OWNER_CITY = ownerCity
        property.OWNER_STATE = ownerState
        property.ZONE = zone
        property.ACRES = parseFloat(acres)

        log.Printf("Successfully scraped main page for PID %s", property.ALPHA)

        return nil
    }
}

// scrapeSalesPage scrapes data from the sales page
func (nhs *NewHanoverScraper) scrapeSalesPage(property *Property) chromedp.ActionFunc {
    // ... existing code ...
    return func(ctx context.Context) error {
        log.Printf("Scraping sales page for PID %s", property.ALPHA)

        timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
        defer cancel()

        var saleDate, salePrice string

        err := chromedp.Run(timeoutCtx,
            chromedp.Click(`//*[@id="sidemenu"]/ul/li[2]/a/span`, chromedp.NodeVisible, chromedp.BySearch),
            chromedp.WaitVisible(`//*[@id="Sale Details"]`, chromedp.BySearch),
            chromedp.Text(`//*[@id="Sale Details"]/tbody/tr[1]/td[2]`, &saleDate, chromedp.BySearch),
            chromedp.Text(`//*[@id="Sale Details"]/tbody/tr[3]/td[2]`, &salePrice, chromedp.BySearch),
        )
        if err != nil {
            log.Printf("Sales data not found for PID %s: %v", property.ALPHA, err)
            // Continue without sales data
            return nil
        }

        log.Printf("Extracted Sales Data: Sale Date=%s, Sale Price=%s", saleDate, salePrice)

        property.SALE_DATE = saleDate
        property.SALE_PRICE = parseFloat(salePrice)

        log.Printf("Successfully scraped sales page for PID %s", property.ALPHA)
        return nil
    }
}

// scrapeCommercialPage scrapes data from the commercial page
func (nhs *NewHanoverScraper) scrapeCommercialPage(property *Property) chromedp.ActionFunc {
    // ... existing code ...
    return func(ctx context.Context) error {
        log.Printf("Scraping commercial page for PID %s", property.ALPHA)

        timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
        defer cancel()

        var yearBuilt, sqft string

        err := chromedp.Run(timeoutCtx,
            chromedp.Click(`//*[@id="sidemenu"]/ul/li[4]/a/span`, chromedp.NodeVisible, chromedp.BySearch),
            chromedp.WaitVisible(`//*[@id="Commercial"]`, chromedp.BySearch),
            chromedp.Text(`//*[@id="Commercial"]/tbody/tr[6]/td[2]`, &yearBuilt, chromedp.BySearch),
            chromedp.Text(`//*[@id="Commercial"]/tbody/tr[12]/td[2]`, &sqft, chromedp.BySearch),
        )
        if err != nil {
            log.Printf("Commercial data not found for PID %s: %v", property.ALPHA, err)
            // Continue without commercial data
            return nil
        }

        log.Printf("Extracted Commercial Data: Year Built=%s, SQFT=%s", yearBuilt, sqft)

        property.YEAR_BUILT = yearBuilt
        property.SQFT = parseFloat(sqft)

        log.Printf("Successfully scraped commercial page for PID %s", property.ALPHA)
        return nil
    }
}

// scrapeValuesPage scrapes data from the values page
func (nhs *NewHanoverScraper) scrapeValuesPage(property *Property) chromedp.ActionFunc {
    // ... existing code ...
    return func(ctx context.Context) error {
        log.Printf("Scraping values page for PID %s", property.ALPHA)

        timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
        defer cancel()

        var appraised string

        err := chromedp.Run(timeoutCtx,
            chromedp.Click(`//*[@id="sidemenu"]/ul/li[8]/a/span`, chromedp.NodeVisible, chromedp.BySearch),
            chromedp.WaitVisible(`//*[@id="Values"]`, chromedp.BySearch),
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

// retry retries a function with exponential backoff
func retry(attempts int, sleep time.Duration, f func() error) error {
    err := f()
    if err == nil {
        return nil
    }

    if attempts--; attempts > 0 {
        log.Printf("Retrying after error: %v", err)
        time.Sleep(sleep)
        return retry(attempts, sleep*2, f)
    }
    return err
}