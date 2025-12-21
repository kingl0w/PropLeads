package sos

import (
	"context"
	"fmt"
	"log"
	"strings"
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
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(1920, 1080),
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

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var info BusinessInfo
	info.BusinessName = companyName

	logger("Starting LookupBusiness for %s", companyName)

	err := chromedp.Run(ctx,
		chromedp.Navigate(`https://www.sosnc.gov/online_services/search/by_title/_Business_Registration`),
		chromedp.Sleep(1*time.Second),
		chromedp.WaitVisible(`#SearchCriteria`, chromedp.ByID),
		chromedp.SendKeys(`#SearchCriteria`, companyName, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),
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
		// Return info with "No match" instead of erroring completely
		info.CompanyOfficials = append(info.CompanyOfficials, Official{Title: "Result", Name: "No match"})
		return info, nil
	}

	logger("LookupBusiness completed for %s, found %d officials", companyName, len(info.CompanyOfficials))
	return info, nil
}

func waitForResultsOrNoResults(ctx context.Context, companyName string, info *BusinessInfo) error {
	// Wait for either results or "no results" message
	var recordsFound string

	// Try multiple selectors for robustness
	selectors := []string{
		`#results-article > div > span:nth-child(1)`,
		`#results-article span`,
		`div[id*="results"] span`,
	}

	var err error
	for _, selector := range selectors {
		err = chromedp.Text(selector, &recordsFound, chromedp.ByQuery).Do(ctx)
		if err == nil {
			break
		}
	}

	if err != nil {
		if VerboseLogging {
			log.Printf("Error getting records found text for %s: %v", companyName, err)
		}
		return err
	}

	// Check if no records were found
	if strings.Contains(recordsFound, "Records Found: 0") || strings.Contains(recordsFound, "0 records") {
		if VerboseLogging {
			log.Printf("No records found for %s", companyName)
		}
		info.CompanyOfficials = append(info.CompanyOfficials, Official{Title: "Result", Name: "No match"})
		return nil
	}

	// Look for the best result (prefer those with Annual Report button)
	var annualReportIndex int
	err = chromedp.Evaluate(`
		(() => {
			const rows = document.querySelectorAll('#results-div > table > tbody > tr, table[id*="result"] > tbody > tr');
			for (let i = 0; i < Math.min(rows.length, 5); i++) {
				const annualReportLink = rows[i].querySelector('a.button[href*="annual_report"], a[href*="annual_report"]');
				if (annualReportLink) {
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
		// Continue anyway, just use the first result
		annualReportIndex = 0
	}

	// Click on the appropriate result
	var clickSelector string
	if annualReportIndex > 0 {
		clickSelector = fmt.Sprintf(`#results-div > table > tbody > tr:nth-child(%d) td:first-child b a`, annualReportIndex)
		if VerboseLogging {
			log.Printf("Clicking on result %d with annual report for %s", annualReportIndex, companyName)
		}
	} else {
		// Try multiple selectors for the first result
		clickSelectors := []string{
			`#results-div > table > tbody > tr:first-child td:first-child b a`,
			`#results-div > table > tbody > tr:first-child a`,
			`table[id*="result"] > tbody > tr:first-child a`,
		}

		for _, sel := range clickSelectors {
			err = chromedp.Click(sel, chromedp.ByQuery).Do(ctx)
			if err == nil {
				clickSelector = sel
				break
			}
		}

		if err != nil {
			if VerboseLogging {
				log.Printf("Error clicking on result for %s: %v", companyName, err)
			}
			return err
		}

		if VerboseLogging {
			log.Printf("Clicked on first result for %s using selector: %s", companyName, clickSelector)
		}

		// Wait for page to load
		err = chromedp.WaitVisible(`#filings-article, article[id*="filing"], div[id*="filing"]`, chromedp.ByQuery).Do(ctx)
		if err != nil {
			if VerboseLogging {
				log.Printf("Error waiting for filings article for %s: %v", companyName, err)
			}
			return err
		}

		return extractOfficials(ctx, info)
	}

	err = chromedp.Click(clickSelector, chromedp.ByQuery).Do(ctx)
	if err != nil {
		if VerboseLogging {
			log.Printf("Error clicking on result for %s: %v", companyName, err)
		}
		return err
	}

	// Wait for the filings page to load - try multiple selectors
	err = chromedp.WaitVisible(`#filings-article, article[id*="filing"], div[id*="filing"]`, chromedp.ByQuery).Do(ctx)
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

	// Try multiple approaches to extract officials for robustness
	var officials []Official

	// Approach 1: Original selector
	err := chromedp.Evaluate(`
		(() => {
			const officials = [];

			// Try multiple section selectors
			const sectionSelectors = [
				'#filings-article > section > section:nth-child(6)',
				'#filings-article section section:nth-child(6)',
				'section:has(span.greenLabel)',
				'section section'
			];

			let section = null;
			for (const selector of sectionSelectors) {
				section = document.querySelector(selector);
				if (section && section.querySelector('span.greenLabel')) {
					break;
				}
			}

			if (section) {
				const paragraphs = section.getElementsByTagName('p');
				for (let p of paragraphs) {
					const titleSpan = p.querySelector('span.greenLabel');
					const nameLink = p.querySelector('span:nth-child(3) > a, a');
					if (titleSpan && nameLink) {
						officials.push({
							title: titleSpan.textContent.trim(),
							name: nameLink.textContent.trim()
						});
					}
				}
			}

			// If no officials found, try alternative structure
			if (officials.length === 0) {
				const allLabels = document.querySelectorAll('span.greenLabel');
				for (const label of allLabels) {
					const parent = label.closest('p');
					if (parent) {
						const link = parent.querySelector('a');
						if (link) {
							officials.push({
								title: label.textContent.trim(),
								name: link.textContent.trim()
							});
						}
					}
				}
			}

			return officials;
		})()
	`, &officials).Do(ctx)

	if err != nil {
		if VerboseLogging {
			log.Printf("Error extracting officials for %s: %v", info.BusinessName, err)
		}
		info.CompanyOfficials = append(info.CompanyOfficials, Official{Title: "Result", Name: "No officials found"})
		return nil
	}

	if len(officials) == 0 {
		info.CompanyOfficials = append(info.CompanyOfficials, Official{Title: "Result", Name: "No officials found"})
		if VerboseLogging {
			log.Printf("No officials found for %s", info.BusinessName)
		}
	} else {
		info.CompanyOfficials = officials
		if VerboseLogging {
			log.Printf("Found %d officials for %s", len(officials), info.BusinessName)
		}
	}

	return nil
}

func RetryTimedOutBusinesses(timedOutBusinesses []string) []BusinessInfo {
	var results []BusinessInfo
	sem := make(chan struct{}, 3) // Reduced concurrency for more stability

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, companyName := range timedOutBusinesses {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire semaphore
			defer func() { <-sem }() // Release semaphore

			info, err := LookupBusiness(name)
			if err == nil && len(info.CompanyOfficials) > 0 && info.CompanyOfficials[0].Name != "Timeout" && info.CompanyOfficials[0].Name != "No match" {
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
