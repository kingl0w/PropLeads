package county

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/kingl0w/PropLeads/internal/scrapeutils"
)

type BrunswickScraper struct{}

// NewBrunswickScraper creates a new scraper instance
func NewBrunswickScraper() *BrunswickScraper {
    return &BrunswickScraper{}
}

// Scrape method to scrape multiple PIDs
func (bs *BrunswickScraper) Scrape(pids []string) ([]Property, error) {
    opts := append(chromedp.DefaultExecAllocatorOptions[:],
        chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64)"),
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
    browserCtx, cancelBrowser := chromedp.NewContext(allocCtx, chromedp.WithLogf(scrapeutils.CustomLogger))
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
            tabCtx, cancelTab := chromedp.NewContext(browserCtx, chromedp.WithLogf(scrapeutils.CustomLogger))
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

                err := bs.scrapePID(tabCtx, pid, &property)
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

// scrapePID scrapes data for a single PID
func (bs *BrunswickScraper) scrapePID(ctx context.Context, pid string, property *Property) error {
    pid = strings.TrimSpace(pid) // Trim whitespace from PID

    property.PIN = pid // Set the PIN field to PID
    property.COUNTY = "Brunswick"

    log.Printf("Starting scrape for PID %s", pid)

    // Create a new context with a timeout for each scrape attempt
    timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
    defer cancel()

    err := chromedp.Run(timeoutCtx,
        chromedp.Navigate("https://tax.brunsco.net/itsnet/RealEstate.aspx"),
        chromedp.ActionFunc(func(ctx context.Context) error {
            log.Printf("Navigated to search page for PID %s", pid)
            return nil
        }),
        bs.searchAndClickResult(pid),
        bs.scrapeMainPage(property),
        bs.scrapeLandPage(property),
        bs.scrapeBuildingPage(property),
        bs.scrapeSalesPage(property),
        bs.scrapeOwnersPage(property),
        bs.scrapeTaxCodesPage(property),
    )

    if err != nil {
        return fmt.Errorf("error scraping PID %s: %v", pid, err)
    }

    log.Printf("Successfully scraped PID %s", pid)
    return nil
}

// searchAndClickResult searches for the PID and clicks the result
func (bs *BrunswickScraper) searchAndClickResult(pid string) chromedp.ActionFunc {
    return func(ctx context.Context) error {
        log.Printf("Entering PID %s into search form", pid)

        err := chromedp.Run(ctx,
            chromedp.WaitVisible(`#ctl00_contentplaceholderRealEstateSearch_usercontrolRealEstateSearch_ctrlParcelNumber_txtPARCEL`, chromedp.ByQuery),
            chromedp.SendKeys(`#ctl00_contentplaceholderRealEstateSearch_usercontrolRealEstateSearch_ctrlParcelNumber_txtPARCEL`, pid, chromedp.ByQuery),
            chromedp.Click(`#ctl00_contentplaceholderRealEstateSearch_usercontrolRealEstateSearch_buttonSearch`, chromedp.ByQuery),
            scrapeutils.WaitVisibleWithTimeout(`#ctl00_contentplaceholderRealEstateSearchResults_usercontrolRealEstateSearchResult_gridviewSearchResults`, chromedp.ByQuery, scrapeutils.ElementWaitTimeout),
            chromedp.Click(`//*[@id="ctl00_contentplaceholderRealEstateSearchResults_usercontrolRealEstateSearchResult_gridviewSearchResults"]/tbody/tr[2]/td[1]/a`, chromedp.BySearch),
            scrapeutils.WaitVisibleWithTimeout(`#ctl00_contentplaceholderRealEstateSearchSummary_usercontrolRealEstateParcelSummaryInfo_labelOwnerName`, chromedp.ByQuery, scrapeutils.ElementWaitTimeout),
        )
        if err != nil {
            return fmt.Errorf("error searching and clicking result: %w", err)
        }

        log.Printf("Clicked search result for PID %s", pid)
        return nil
    }
}

// scrapeMainPage scrapes data from the main page
func (bs *BrunswickScraper) scrapeMainPage(property *Property) chromedp.ActionFunc {
    return func(ctx context.Context) error {
        log.Printf("Scraping main page for PID %s", property.PIN)

        var ownerName string

        err := chromedp.Run(ctx,
            chromedp.Text(`#ctl00_contentplaceholderRealEstateSearchSummary_usercontrolRealEstateParcelSummaryInfo_labelOwnerName`, &ownerName, chromedp.ByQuery),
        )
        if err != nil {
            return fmt.Errorf("error scraping main page: %w", err)
        }

        log.Printf("Extracted Data: Owner=%s", ownerName)

        // Set the extracted owner name to the NAME field of the property
        property.NAME = strings.TrimSpace(ownerName)

        log.Printf("Successfully scraped main page for PID %s", property.PIN)

        return nil
    }
}

// scrapeLandPage scrapes data from the land page
func (bs *BrunswickScraper) scrapeLandPage(property *Property) chromedp.ActionFunc {
    return func(ctx context.Context) error {
        log.Printf("Scraping land page for PID %s", property.PIN)

        var acresStr, zone, appraisedStr string

        err := chromedp.Run(ctx,
            chromedp.Text(`//*[@id="ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelLand_usercontrolRealEstateParcelLandData_gridviewParcelMarketLandData"]/tbody/tr[3]/td[12]`, &acresStr, chromedp.BySearch),
            chromedp.Text(`//*[@id="ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelLand_usercontrolRealEstateParcelLandData_gridviewParcelMarketLandData"]/tbody/tr[3]/td[3]`, &zone, chromedp.BySearch),
            chromedp.Text(`//*[@id="ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelLand_usercontrolRealEstateParcelLandData_gridviewParcelMarketLandData"]/tbody/tr[3]/td[16]`, &appraisedStr, chromedp.BySearch),
        )
        if err != nil {
            log.Printf("Land data not found for PID %s: %v", property.PIN, err)
            // Continue without land data
            return nil
        }

        log.Printf("Extracted Land Data: Acres=%s, Zone=%s, Appraised=%s", acresStr, zone, appraisedStr)

        property.ACRES = scrapeutils.ParseFloat(acresStr)
        property.ZONE = strings.TrimSpace(zone)
        property.APPRAISED = scrapeutils.ParseFloat(appraisedStr)

        log.Printf("Successfully scraped land page for PID %s", property.PIN)
        return nil
    }
}

// scrapeBuildingPage scrapes data from the building page
func (bs *BrunswickScraper) scrapeBuildingPage(property *Property) chromedp.ActionFunc {
    return func(ctx context.Context) error {
        log.Printf("Scraping building page for PID %s", property.PIN)

        // Click on the Building tab
        err := chromedp.Run(ctx,
            chromedp.Click(`#__tab_ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelBuilding`, chromedp.ByQuery),
            scrapeutils.WaitVisibleWithTimeout(`#ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelBuilding_usercontrolRealEstateParcelBuildingData_gridviewParcelBuilding`, chromedp.ByQuery, scrapeutils.ElementWaitTimeout),
        )
        if err != nil {
            log.Printf("Building tab not found for PID %s: %v", property.PIN, err)
            // Continue without building data
            return nil
        }

        var sqftStr, yearBuilt string

        err = chromedp.Run(ctx,
            chromedp.Text(`//*[@id="ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelBuilding_usercontrolRealEstateParcelBuildingData_gridviewParcelBuilding"]/tbody/tr[2]/td[6]`, &sqftStr, chromedp.BySearch),
            chromedp.Text(`//*[@id="ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelBuilding_usercontrolRealEstateParcelBuildingData_gridviewParcelBuilding"]/tbody/tr[2]/td[2]`, &yearBuilt, chromedp.BySearch),
        )
        if err != nil {
            log.Printf("Building data not found for PID %s: %v", property.PIN, err)
            // Continue without building data
            return nil
        }

        log.Printf("Extracted Building Data: SQFT=%s, Year Built=%s", sqftStr, yearBuilt)

        property.SQFT = scrapeutils.ParseFloat(sqftStr)
        property.YEAR_BUILT = strings.TrimSpace(yearBuilt)

        log.Printf("Successfully scraped building page for PID %s", property.PIN)
        return nil
    }
}

// scrapeSalesPage scrapes data from the sales page
func (bs *BrunswickScraper) scrapeSalesPage(property *Property) chromedp.ActionFunc {
    return func(ctx context.Context) error {
        log.Printf("Scraping sales page for PID %s", property.PIN)

        // Click on the Sales tab
        err := chromedp.Run(ctx,
            chromedp.Click(`#__tab_ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelSales`, chromedp.ByQuery),
            scrapeutils.WaitVisibleWithTimeout(`#ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelSales_usercontrolRealEstateParcelSalesData_gridviewParcelSalesData`, chromedp.ByQuery, scrapeutils.ElementWaitTimeout),
        )
        if err != nil {
            log.Printf("Sales tab not found for PID %s: %v", property.PIN, err)
            // Continue without sales data
            return nil
        }

        var saleDate, salePrice string

        // Find the number of rows in the sales table
        var salesTableHTML string
        err = chromedp.OuterHTML(`#ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelSales_usercontrolRealEstateParcelSalesData_gridviewParcelSalesData`, &salesTableHTML, chromedp.ByQuery).Do(ctx)
        if err != nil {
            log.Printf("Error getting sales table HTML for PID %s: %v", property.PIN, err)
            return nil
        }

        // Count the number of rows in the sales table
        rowsCount := strings.Count(salesTableHTML, "<tr")

        if rowsCount < 3 {
            log.Printf("No sales data available for PID %s", property.PIN)
            return nil
        }

        // The most recent sale is usually at the bottom
        saleDateXPath := fmt.Sprintf(`//*[@id="ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelSales_usercontrolRealEstateParcelSalesData_gridviewParcelSalesData"]/tbody/tr[%d]/td[3]`, rowsCount)
        salePriceXPath := fmt.Sprintf(`//*[@id="ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelSales_usercontrolRealEstateParcelSalesData_gridviewParcelSalesData"]/tbody/tr[%d]/td[6]`, rowsCount)

        err = chromedp.Run(ctx,
            chromedp.Text(saleDateXPath, &saleDate, chromedp.BySearch),
            chromedp.Text(salePriceXPath, &salePrice, chromedp.BySearch),
        )
        if err != nil {
            log.Printf("Error extracting sales data for PID %s: %v", property.PIN, err)
            return nil
        }

        log.Printf("Extracted Sales Data: Sale Date=%s, Sale Price=%s", saleDate, salePrice)

        property.SALE_DATE = strings.TrimSpace(saleDate)
        property.SALE_PRICE = scrapeutils.ParseFloat(salePrice)

        log.Printf("Successfully scraped sales page for PID %s", property.PIN)
        return nil
    }
}

// scrapeOwnersPage scrapes data from the owners page
func (bs *BrunswickScraper) scrapeOwnersPage(property *Property) chromedp.ActionFunc {
    return func(ctx context.Context) error {
        log.Printf("Scraping owners page for PID %s", property.PIN)

        // Click on the Owners tab
        err := chromedp.Run(ctx,
            chromedp.Click(`#__tab_ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelOwners`, chromedp.ByQuery),
            scrapeutils.WaitVisibleWithTimeout(`#ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelOwners_usercontrolRealEstateParcelOwnersData_labelMailingAddress2Value`, chromedp.ByQuery, scrapeutils.ElementWaitTimeout),
        )
        if err != nil {
            log.Printf("Owners tab not found for PID %s: %v", property.PIN, err)
            // Continue without owners data
            return nil
        }

        var ownerAddress, ownerCity, ownerState string

        err = chromedp.Run(ctx,
            chromedp.Text(`#ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelOwners_usercontrolRealEstateParcelOwnersData_labelMailingAddress2Value`, &ownerAddress, chromedp.ByQuery),
            chromedp.Text(`#ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelOwners_usercontrolRealEstateParcelOwnersData_labelMailingAddressCityValue`, &ownerCity, chromedp.ByQuery),
            chromedp.Text(`#ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelOwners_usercontrolRealEstateParcelOwnersData_labelMailingAddressStateValue`, &ownerState, chromedp.ByQuery),
        )
        if err != nil {
            log.Printf("Owners data not found for PID %s: %v", property.PIN, err)
            // Continue without owners data
            return nil
        }

        log.Printf("Extracted Owners Data: Owner Address=%s, Owner City=%s, Owner State=%s", ownerAddress, ownerCity, ownerState)

        property.OWNER_ADDRESS = strings.TrimSpace(ownerAddress)
        property.OWNER_CITY = strings.TrimSpace(ownerCity)
        property.OWNER_STATE = strings.TrimSpace(ownerState)

        log.Printf("Successfully scraped owners page for PID %s", property.PIN)
        return nil
    }
}

// scrapeTaxCodesPage scrapes data from the tax codes page
func (bs *BrunswickScraper) scrapeTaxCodesPage(property *Property) chromedp.ActionFunc {
    return func(ctx context.Context) error {
        log.Printf("Scraping tax codes page for PID %s", property.PIN)

        // Click on the Tax Codes tab
        err := chromedp.Run(ctx,
            chromedp.Click(`#__tab_ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelTaxCodes`, chromedp.ByQuery),
            scrapeutils.WaitVisibleWithTimeout(`#ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelTaxCodes_usercontrolRealEstateParcelTaxCodes_gridviewParcelTaxCodesData`, chromedp.ByQuery, scrapeutils.ElementWaitTimeout),
        )
        if err != nil {
            log.Printf("Tax Codes tab not found for PID %s: %v", property.PIN, err)
            // Continue without tax codes data
            return nil
        }

        var taxCode, propertyCity, township string

        err = chromedp.Run(ctx,
            chromedp.Text(`#ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelTaxCodes_usercontrolRealEstateParcelTaxCodes_gridviewParcelTaxCodesData > tbody > tr.RowStyleDefaultGridViewSkin > td:nth-child(2)`, &taxCode, chromedp.ByQuery),
            chromedp.Text(`//*[@id="ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelTaxCodes_usercontrolRealEstateParcelTaxCodes_gridviewParcelLocationCodesData"]/tbody/tr[5]/td[3]`, &propertyCity, chromedp.BySearch),
            chromedp.Text(`//*[@id="ctl00_contentplaceholderRealEstateWorkplace_tabcontainerWorkSpace_tabpanelTaxCodes_usercontrolRealEstateParcelTaxCodes_gridviewParcelLocationCodesData"]/tbody/tr[4]/td[3]`, &township, chromedp.BySearch),
        )
        if err != nil {
            log.Printf("Tax Codes data not found for PID %s: %v", property.PIN, err)
            // Continue without tax codes data
            return nil
        }

        log.Printf("Extracted Tax Codes Data: Tax Code=%s, Property City=%s, Township=%s", taxCode, propertyCity, township)

        property.TAX_CODES = strings.TrimSpace(taxCode)
        property.PROPERTY_CITY = strings.TrimSpace(propertyCity)
        property.TOWNSHIP = strings.TrimSpace(township)

        log.Printf("Successfully scraped tax codes page for PID %s", property.PIN)
        return nil
    }
}
