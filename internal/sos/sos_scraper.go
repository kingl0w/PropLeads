package sos

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// VerboseLogging controls the amount of logging output
var VerboseLogging bool

type Official struct {
    Title string `json:"title"`
    Name  string `json:"name"`
}

type BusinessInfo struct {
    BusinessName     string     `json:"business_name"`
    CompanyOfficials []Official `json:"company_officials"`
}

func LookupBusiness(companyName string) (BusinessInfo, error) {
    opts := append(chromedp.DefaultExecAllocatorOptions[:],
        chromedp.Flag("ignore-certificate-errors", true),
        chromedp.Flag("disable-web-security", true),
        chromedp.Flag("disable-gpu", true),
        chromedp.Flag("no-sandbox", true),
        chromedp.Flag("headless", true),
    )

    allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
    defer cancel()

    // Use a no-op logger if verbose logging is disabled
    logger := log.Printf
    if !VerboseLogging {
        logger = func(format string, args ...interface{}) {}
    }

    ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(logger))
    defer cancel()

    ctx, cancel = context.WithTimeout(ctx, 20*time.Second)
    defer cancel()

    var info BusinessInfo
    info.BusinessName = companyName

    logger("Starting LookupBusiness for %s", companyName)

    err := chromedp.Run(ctx,
        chromedp.Navigate(`https://www.sosnc.gov/online_services/search/by_title/_Business_Registration`),
        chromedp.WaitVisible(`#SearchCriteria`, chromedp.ByID),
        chromedp.SendKeys(`#SearchCriteria`, companyName, chromedp.ByID),
        chromedp.Click(`#SubmitButton`, chromedp.ByID),
        chromedp.Sleep(2*time.Second),
        chromedp.ActionFunc(func(ctx context.Context) error {
            return waitForResultsOrNoResults(ctx, companyName, &info)
        }),
    )

    if err != nil {
        if err == context.DeadlineExceeded {
            if VerboseLogging {
                log.Printf("Process took too long for %s", companyName)
            }
            info.CompanyOfficials = append(info.CompanyOfficials, Official{Title: "Result", Name: "No match"})
            return info, nil
        }
        if VerboseLogging {
            log.Printf("Error occurred while processing %s: %v", companyName, err)
        }
        return BusinessInfo{}, fmt.Errorf("error scraping business info: %v", err)
    }

    logger("LookupBusiness completed for %s, found %d officials", companyName, len(info.CompanyOfficials))
    return info, nil
}

func waitForResultsOrNoResults(ctx context.Context, companyName string, info *BusinessInfo) error {
    var recordsFound string
    err := chromedp.Text(`#results-article > div > span:nth-child(1)`, &recordsFound).Do(ctx)
    if err != nil {
        if VerboseLogging {
            log.Printf("Error getting records found text for %s: %v", companyName, err)
        }
        return err
    }

    if recordsFound == "Records Found: 0" {
        if VerboseLogging {
            log.Printf("No records found for %s", companyName)
        }
        info.CompanyOfficials = append(info.CompanyOfficials, Official{Title: "Result", Name: "No match"})
        return nil
    }

    // Check for Annual Report button in top 3 results
    var annualReportIndex int
    err = chromedp.Evaluate(`
        (() => {
            const rows = document.querySelectorAll('#results-div > table > tbody > tr');
            for (let i = 0; i < Math.min(rows.length, 3); i++) {
                if (rows[i].querySelector('a.button[href*="annual_report"]')) {
                    return i + 1;
                }
            }
            return 0;
        })()
    `, &annualReportIndex).Do(ctx)
    if err != nil {
        if VerboseLogging {
            log.Printf("Error evaluating annual report index for %s: %v", companyName, err)
        }
        return err
    }

    // Click on the appropriate result
    var clickSelector string
    if annualReportIndex > 0 {
        clickSelector = fmt.Sprintf(`#results-div > table > tbody > tr:nth-child(%d) td:first-child b a`, annualReportIndex)
        if VerboseLogging {
            log.Printf("Clicking on result %d with annual report for %s", annualReportIndex, companyName)
        }
    } else {
        clickSelector = `#results-div > table > tbody > tr:first-child td:first-child b a`
        if VerboseLogging {
            log.Printf("Clicking on first result for %s", companyName)
        }
    }

    err = chromedp.Click(clickSelector, chromedp.ByQuery).Do(ctx)
    if err != nil {
        if VerboseLogging {
            log.Printf("Error clicking on result for %s: %v", companyName, err)
        }
        return err
    }

    err = chromedp.WaitVisible(`#filings-article`, chromedp.ByID).Do(ctx)
    if err != nil {
        if VerboseLogging {
            log.Printf("Error waiting for filings article for %s: %v", companyName, err)
        }
        return err
    }

    return extractOfficials(ctx, info)
}

func extractOfficials(ctx context.Context, info *BusinessInfo) error {
    if VerboseLogging {
        log.Printf("Extracting officials for %s", info.BusinessName)
    }

    err := chromedp.Evaluate(`
        (() => {
            const officials = [];
            const section = document.querySelector('#filings-article > section > section:nth-child(6)');
            if (section) {
                const paragraphs = section.getElementsByTagName('p');
                for (let p of paragraphs) {
                    const titleSpan = p.querySelector('span.greenLabel');
                    const nameLink = p.querySelector('span:nth-child(3) > a');
                    if (titleSpan && nameLink) {
                        officials.push({
                            title: titleSpan.textContent.trim(),
                            name: nameLink.textContent.trim()
                        });
                    }
                }
            }
            return officials;
        })()
    `, &info.CompanyOfficials).Do(ctx)

    if err != nil {
        if VerboseLogging {
            log.Printf("Error extracting officials for %s: %v", info.BusinessName, err)
        }
        return err
    }

    if len(info.CompanyOfficials) == 0 {
        info.CompanyOfficials = append(info.CompanyOfficials, Official{Title: "Result", Name: "No officials found"})
        if VerboseLogging {
            log.Printf("No officials found for %s", info.BusinessName)
        }
    } else if VerboseLogging {
        log.Printf("Found %d officials for %s", len(info.CompanyOfficials), info.BusinessName)
    }

    return nil
}

func RetryTimedOutBusinesses(timedOutBusinesses []string) []BusinessInfo {
    var results []BusinessInfo
    sem := make(chan struct{}, 5) // Limit concurrency to 5 goroutines

    var mu sync.Mutex
    var wg sync.WaitGroup

    for _, companyName := range timedOutBusinesses {
        wg.Add(1)
        go func(name string) {
            defer wg.Done()
            sem <- struct{}{}        // Acquire semaphore
            defer func() { <-sem }() // Release semaphore

            info, err := LookupBusiness(name)
            if err == nil && len(info.CompanyOfficials) > 0 && info.CompanyOfficials[0].Name != "Timeout" {
                mu.Lock()
                results = append(results, info)
                mu.Unlock()
                if VerboseLogging {
                    log.Printf("Retry successful for %s", name)
                }
            } else if VerboseLogging {
                log.Printf("Retry failed for %s: %v", name, err)
            }
        }(companyName)
    }

    wg.Wait()
    return results
}
