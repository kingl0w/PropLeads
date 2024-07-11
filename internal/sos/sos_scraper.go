package sos

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/chromedp"
)

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

    ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
    defer cancel()

    // Set a timeout for the entire operation
    ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    var info BusinessInfo
    info.BusinessName = companyName

    err := chromedp.Run(ctx,
        chromedp.Navigate(`https://www.sosnc.gov/online_services/search/by_title/_Business_Registration`),
        chromedp.WaitVisible(`#SearchCriteria`, chromedp.ByID),
        chromedp.SendKeys(`#SearchCriteria`, companyName, chromedp.ByID),
        chromedp.Click(`#SubmitButton`, chromedp.ByID),
        chromedp.ActionFunc(func(ctx context.Context) error {
            log.Printf("Clicked search button for %s", companyName)
            return nil
        }),
        chromedp.ActionFunc(func(ctx context.Context) error {
            return waitForResultsOrNoResults(ctx)
        }),
        chromedp.ActionFunc(func(ctx context.Context) error {
            var noResults int
            if err := chromedp.Evaluate(`document.querySelector('#results-div').textContent.includes('No records found') ? 1 : 0`, &noResults).Do(ctx); err != nil {
                return err
            }
            if noResults == 1 {
                log.Printf("No results found for %s", companyName)
                return nil
            }
            if err := chromedp.Click(`#results-div table tbody tr:first-child td:first-child b a`, chromedp.ByQuery).Do(ctx); err != nil {
                return err
            }
            return waitForElement(ctx, `#filings-article > section > section:nth-child(6)`)
        }),
        chromedp.Evaluate(`
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
        `, &info.CompanyOfficials),
    )

    if err != nil {
        if err == context.DeadlineExceeded {
            log.Printf("Timeout occurred while processing %s", companyName)
            info.CompanyOfficials = append(info.CompanyOfficials, Official{Title: "Result", Name: "Timeout"})
            return info, nil
        }
        log.Printf("Error occurred while processing %s: %v", companyName, err)
        return BusinessInfo{}, fmt.Errorf("error scraping business info: %v", err)
    }

    if len(info.CompanyOfficials) == 0 {
        info.CompanyOfficials = append(info.CompanyOfficials, Official{Title: "Result", Name: "No match"})
    }

    return info, nil
}

func waitForResultsOrNoResults(ctx context.Context) error {
    return chromedp.WaitVisible(`#results-div`, chromedp.ByID).Do(ctx)
}

func waitForElement(ctx context.Context, selector string) error {
    return chromedp.WaitVisible(selector, chromedp.ByQuery).Do(ctx)
}