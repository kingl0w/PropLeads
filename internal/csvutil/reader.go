package csv

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/kingl0w/PropLeads/internal/dataprocessing"
)

func ReadPIDs(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	pids := make([]string, 0, len(records))
	for _, record := range records {
		if len(record) > 0 {
			pids = append(pids, record[0])
		}
	}
	return pids, nil
}

func ReadParcelResults(filename string) ([]map[string]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable number of fields per record

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %v", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("file contains insufficient data: found %d row(s)", len(records))
	}

	expectedHeaders := dataprocessing.HeadersConfig.ParcelResults
	actualHeaders := records[0]

	fmt.Printf("Expected headers: %v\n", expectedHeaders)
	fmt.Printf("Actual headers: %v\n", actualHeaders)

	if len(actualHeaders) != len(expectedHeaders) {
		return nil, fmt.Errorf("header mismatch: expected %d fields %v, got %d fields %v", 
			len(expectedHeaders), expectedHeaders, len(actualHeaders), actualHeaders)
	}

    results := make([]map[string]string, 0, len(records)-1)

    for i, record := range records[1:] {
        if len(record) != len(actualHeaders) {
            return nil, fmt.Errorf("mismatch in number of fields on line %d: expected %d, got %d. Data: %v", 
                i+2, len(actualHeaders), len(record), record)
        }

        result := make(map[string]string, len(actualHeaders))
        for j, value := range record {
            key := actualHeaders[j]
            value = strings.TrimSpace(value)
            value = strings.Trim(value, "\"") // Remove surrounding quotes
            result[key] = value
        }
        results = append(results, result)
    }
    return results, nil
}

func IsBusinessName(name string) bool {
	businessIndicators := []string{"LLC", "INC", "CORP", "LTD", "COMPANY", "PROPERTIES", "ENTERPRISES"}
	upperName := strings.ToUpper(name)
	for _, indicator := range businessIndicators {
		if strings.Contains(upperName, indicator) {
			return true
		}
	}
	return false
}