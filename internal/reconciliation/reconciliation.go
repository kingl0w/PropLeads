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

    if len(unifiedResults) == 0 {
        return fmt.Errorf("unified results file is empty")
    }

    fmt.Printf("Unified results headers: %v\n", unifiedResults[0])

    wpResults, err := readWPResults(wpSearchPath)
    if err != nil {
        return fmt.Errorf("error reading WP search results: %v", err)
    }

    fmt.Printf("Number of WP search results: %d\n", len(wpResults))

    reconciledData := reconcileContactInfo(unifiedResults, wpResults)

    fmt.Printf("Number of reconciled records: %d\n", len(reconciledData)-1)

    err = writeReconciledData(outputPath, reconciledData)
    if err != nil {
        return fmt.Errorf("error writing reconciled data: %v", err)
    }

    return nil
}

func reconcileContactInfo(unifiedResults [][]string, wpResults []ContactInfo) [][]string {
    wpMap := make(map[string]ContactInfo)
    for _, wp := range wpResults {
        normalizedName := normalizeNameForMatching(wp.Name)
        wpMap[normalizedName] = wp
    }

    var reconciledData [][]string
    reconciledData = append(reconciledData, append(unifiedResults[0], "Phones", "Emails"))

    ownerIndex := indexOf(unifiedResults[0], "Owner")
    businessNameIndex := indexOf(unifiedResults[0], "Business Name")
    officialNameIndex := indexOf(unifiedResults[0], "Official Name")

    for _, row := range unifiedResults[1:] {
        owner := row[ownerIndex]
        businessName := ""
        if businessNameIndex != -1 && businessNameIndex < len(row) {
            businessName = row[businessNameIndex]
        }
        officialName := ""
        if officialNameIndex != -1 && officialNameIndex < len(row) {
            officialName = row[officialNameIndex]
        }

        normalizedOwner := normalizeNameForMatching(owner)
        normalizedBusinessName := normalizeNameForMatching(businessName)
        normalizedOfficialName := normalizeNameForMatching(officialName)

        var matchedWP ContactInfo
        var found bool

        if wp, ok := wpMap[normalizedBusinessName]; ok && businessName != "" {
            matchedWP = wp
            found = true
        } else if wp, ok := wpMap[normalizedOwner]; ok {
            matchedWP = wp
            found = true
        } else if wp, ok := wpMap[normalizedOfficialName]; ok && officialName != "" {
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

func readUnifiedResults(path string) ([][]string, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, fmt.Errorf("failed to open unified results file: %v", err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    reader.FieldsPerRecord = -1
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
    reader.FieldsPerRecord = -1
    rows, err := reader.ReadAll()
    if err != nil {
        return nil, fmt.Errorf("failed to read WP results: %v", err)
    }

    if len(rows) < 2 {
        return nil, fmt.Errorf("WP results file is empty or missing header")
    }

    var results []ContactInfo
    for _, row := range rows[1:] {
        if len(row) < 3 {
            continue
        }
        info := ContactInfo{
            Name:   row[0],
            Phones: []string{},
            Emails: []string{},
        }

        for _, field := range row {
            info.Phones = append(info.Phones, extractPhones(field)...)
            info.Emails = append(info.Emails, extractEmails(field)...)
        }

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

func normalizeNameForMatching(name string) string {
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

    pinIndex := indexOf(data[0], "PIN")

    header := make([]string, 0, len(data[0])-1)
    for i, field := range data[0] {
        if i != pinIndex {
            header = append(header, field)
        }
    }
    if err := writer.Write(header); err != nil {
        return err
    }

    for _, row := range data[1:] {
        newRow := make([]string, 0, len(row)-1)
        for i, field := range row {
            if i != pinIndex {
                newRow = append(newRow, field)
            }
        }
        if err := writer.Write(newRow); err != nil {
            return err
        }
    }

    return nil
}

func indexOf(slice []string, item string) int {
    for i, s := range slice {
        if s == item {
            return i
        }
    }
    return -1
}

func extractPhones(s string) []string {
    phoneRegex := regexp.MustCompile(`\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`)
    return phoneRegex.FindAllString(s, -1)
}

func extractEmails(s string) []string {
    emailRegex := regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
    return emailRegex.FindAllString(s, -1)
}