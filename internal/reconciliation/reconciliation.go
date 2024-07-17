package reconciliation

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type ContactInfo struct {
    Name   string
    Phones []string
    Emails []string
}

func ReconcileData(unifiedResultsPath, wpSearchPath, outputPath string) error {
    unifiedResults, err := readUnifiedResults(unifiedResultsPath)
    if err != nil {
        return fmt.Errorf("error reading unified results: %v", err)
    }

    wpResults, err := readWPResults(wpSearchPath)
    if err != nil {
        return fmt.Errorf("error reading WP search results: %v", err)
    }

    reconciledData := reconcileContactInfo(unifiedResults, wpResults)

    err = writeReconciledData(outputPath, reconciledData)
    if err != nil {
        return fmt.Errorf("error writing reconciled data: %v", err)
    }

    return nil
}

func readUnifiedResults(path string) ([][]string, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, fmt.Errorf("failed to open unified results file: %v", err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    records, err := reader.ReadAll()
    if err != nil {
        return nil, fmt.Errorf("failed to read unified results: %v", err)
    }

    if len(records) < 2 {
        return nil, fmt.Errorf("unified results file is empty or missing header")
    }

    return records, nil
}

func readWPResults(path string) ([]ContactInfo, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, fmt.Errorf("failed to open WP results file: %v", err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    reader.FieldsPerRecord = -1 // Allow variable number of fields per record
    rows, err := reader.ReadAll()
    if err != nil {
        return nil, fmt.Errorf("failed to read WP results: %v", err)
    }

    if len(rows) < 2 {
        return nil, fmt.Errorf("WP results file is empty or missing header")
    }

    var results []ContactInfo
    for _, row := range rows[1:] { // Skip header
        if len(row) < 3 {
            continue // Skip rows with insufficient data
        }
        info := ContactInfo{
            Name:   row[0],
            Phones: []string{},
            Emails: []string{},
        }

        // Extract phone numbers and emails from all fields
        for _, field := range row {
            info.Phones = append(info.Phones, extractPhones(field)...)
            info.Emails = append(info.Emails, extractEmails(field)...)
        }

        // Remove duplicates
        info.Phones = removeDuplicates(info.Phones)
        info.Emails = removeDuplicates(info.Emails)

        results = append(results, info)
    }
    return results, nil
}

func removeDuplicates(slice []string) []string {
    keys := make(map[string]bool)
    list := []string{}
    for _, entry := range slice {
        if _, value := keys[entry]; !value {
            keys[entry] = true
            list = append(list, entry)
        }
    }
    return list
}

func reconcileContactInfo(unifiedResults [][]string, wpResults []ContactInfo) [][]string {
    wpMap := make(map[string]ContactInfo)
    for _, wp := range wpResults {
        // Normalize the name for better matching
        normalizedName := normalizeNameForMatching(wp.Name)
        wpMap[normalizedName] = wp
    }

    var reconciledData [][]string
    reconciledData = append(reconciledData, append(unifiedResults[0], "Phones", "Emails"))

    for _, row := range unifiedResults[1:] {
        name := row[1] // Assuming the name is in the second column
        officialName := row[8] // Assuming the official name is in the 9th column
        
        // Try matching with both name and official name
        normalizedName := normalizeNameForMatching(name)
        normalizedOfficialName := normalizeNameForMatching(officialName)
        
        var matchedWP ContactInfo
        var found bool
        
        if wp, ok := wpMap[normalizedName]; ok {
            matchedWP = wp
            found = true
        } else if wp, ok := wpMap[normalizedOfficialName]; ok {
            matchedWP = wp
            found = true
        }

        if found {
            row = append(row, strings.Join(matchedWP.Phones, "; "), strings.Join(matchedWP.Emails, "; "))
        } else {
            row = append(row, "", "")
        }
        reconciledData = append(reconciledData, row)
    }

    return reconciledData
}

func normalizeNameForMatching(name string) string {
    // Remove common prefixes and suffixes, convert to lowercase, and remove punctuation
    name = strings.ToLower(name)
    name = strings.TrimSpace(name)
    name = strings.TrimPrefix(name, "mr ")
    name = strings.TrimPrefix(name, "mrs ")
    name = strings.TrimPrefix(name, "ms ")
    name = strings.TrimPrefix(name, "dr ")
    name = strings.TrimSuffix(name, " jr")
    name = strings.TrimSuffix(name, " sr")
    name = strings.TrimSuffix(name, " iii")
    name = strings.TrimSuffix(name, " ii")
    name = strings.Map(func(r rune) rune {
        if r == ',' || r == '.' || r == '\'' || r == '"' {
            return -1
        }
        return r
    }, name)
    return name
}

func writeReconciledData(path string, data [][]string) error {
    file, err := os.Create(path)
    if err != nil {
        return err
    }
    defer file.Close()

    writer := csv.NewWriter(file)
    defer writer.Flush()

    return writer.WriteAll(data)
}

func extractPhones(s string) []string {
    phoneRegex := regexp.MustCompile(`\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`)
    return phoneRegex.FindAllString(s, -1)
}

func extractEmails(s string) []string {
    emailRegex := regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
    return emailRegex.FindAllString(s, -1)
}