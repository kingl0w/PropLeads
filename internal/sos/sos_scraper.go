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

//calls python seleniumbase scraper with xvfb for cloudflare bypass
func LookupBusiness(companyName string) (BusinessInfo, error) {
	var info BusinessInfo
	info.BusinessName = companyName

	cmd := exec.Command("timeout", "120", "python3", "sos_scraper_seleniumbase.py", companyName)

	output, err := cmd.Output()
	if err != nil && len(output) == 0 {
		return info, fmt.Errorf("seleniumbase scraper failed: %w", err)
	}

	jsonStart := bytes.IndexByte(output, '{')
	jsonEnd := bytes.LastIndexByte(output, '}')

	if jsonStart == -1 || jsonEnd == -1 || jsonStart > jsonEnd {
		return info, fmt.Errorf("no valid JSON found in output: %s", string(output))
	}

	jsonData := output[jsonStart : jsonEnd+1]

	if err := json.Unmarshal(jsonData, &info); err != nil {
		return info, fmt.Errorf("failed to parse scraper output: %w\nJSON: %s", err, string(jsonData))
	}

	return info, nil
}
