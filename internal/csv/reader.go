package csv

import (
	"encoding/csv"
	"os"
	"strings"
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

    var pids []string
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
        return nil, err
    }
    defer file.Close()

    reader := csv.NewReader(file)
    records, err := reader.ReadAll()
    if err != nil {
        return nil, err
    }

    var results []map[string]string
    headers := records[0]

    for _, record := range records[1:] {
        result := make(map[string]string)
        for i, value := range record {
            result[headers[i]] = value
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