package sos

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type BusinessInfo struct {
    BusinessName     string
    CompanyOfficials []string
}

func LookupBusiness(companyName string) (BusinessInfo, error) {
    client := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skip SSL verification
        },
    }

    // Perform the search
    searchURL := "https://www.sosnc.gov/online_services/search/Business_Registration_Results"
    data := url.Values{}
    data.Set("SearchType", "CORPORATION")
    data.Set("SearchCriteria", "NAME")
    data.Set("SearchValue", companyName)

    resp, err := client.PostForm(searchURL, data)
    if err != nil {
        return BusinessInfo{}, fmt.Errorf("error making POST request: %v", err)
    }
    defer resp.Body.Close()

    doc, err := goquery.NewDocumentFromReader(resp.Body)
    if err != nil {
        return BusinessInfo{}, fmt.Errorf("error parsing HTML: %v", err)
    }

    var info BusinessInfo
    info.BusinessName = companyName

    // Find the first result link
    profileLink, exists := doc.Find(".FilingsTable tbody tr:first-child a.java_link").Attr("href")
    if !exists {
        return info, nil // No results found
    }

    // Visit the profile page
    profileURL := "https://www.sosnc.gov" + profileLink
    profileResp, err := client.Get(profileURL)
    if err != nil {
        return BusinessInfo{}, fmt.Errorf("error fetching profile: %v", err)
    }
    defer profileResp.Body.Close()

    profileDoc, err := goquery.NewDocumentFromReader(profileResp.Body)
    if err != nil {
        return BusinessInfo{}, fmt.Errorf("error parsing profile HTML: %v", err)
    }

    // Extract company officials
    profileDoc.Find("section").Each(func(i int, s *goquery.Selection) {
        if strings.Contains(s.Find("header").Text(), "Company Officials") {
            s.Find("p").Each(func(j int, p *goquery.Selection) {
                title := p.Find("span.greenLabel").Text()
                name := p.Find("span > a").Text()
                if title != "" && name != "" {
                    info.CompanyOfficials = append(info.CompanyOfficials, fmt.Sprintf("%s: %s", strings.TrimSpace(title), strings.TrimSpace(name)))
                }
            })
        }
    })

    return info, nil
}