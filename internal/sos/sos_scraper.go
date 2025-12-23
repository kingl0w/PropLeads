package sos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

type Official struct {
	Title string `json:"title"`
	Name  string `json:"name"`
	City  string `json:"city"`
}

type BusinessInfo struct {
	BusinessName     string     `json:"business_name"`
	CompanyOfficials []Official `json:"company_officials"`
}

// LookupBusiness searches for a business on the NC SOS website using SeleniumBase UC Mode
// Falls back to botasaurus scraper if needed (sos_scraper_botasaurus.py is kept as backup)
func LookupBusiness(companyName string) (BusinessInfo, error) {
	var info BusinessInfo
	info.BusinessName = companyName

	// Execute the Python SeleniumBase scraper with a 2-minute timeout
	// Uses UC Mode with xvfb for headless-like operation and Cloudflare bypass
	cmd := exec.Command("timeout", "120", "python3", "sos_scraper_seleniumbase.py", companyName)

	// Capture output (stdout only, stderr goes to console)
	output, err := cmd.Output()
	if err != nil && len(output) == 0 {
		return info, fmt.Errorf("seleniumbase scraper failed: %w", err)
	}

	// Extract JSON from output (scraper prints extra status messages)
	// Find the first { and last } to extract the JSON object
	jsonStart := bytes.IndexByte(output, '{')
	jsonEnd := bytes.LastIndexByte(output, '}')

	if jsonStart == -1 || jsonEnd == -1 || jsonStart > jsonEnd {
		return info, fmt.Errorf("no valid JSON found in output: %s", string(output))
	}

	jsonData := output[jsonStart : jsonEnd+1]

	// Parse JSON output from the Python scraper
	if err := json.Unmarshal(jsonData, &info); err != nil {
		return info, fmt.Errorf("failed to parse scraper output: %w\nJSON: %s", err, string(jsonData))
	}

	return info, nil
}
