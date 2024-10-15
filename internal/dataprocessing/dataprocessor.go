package dataprocessing

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type UnifiedRecord struct {
    ID              string
    PIN             string
    Name            string
    BusinessName    string
    PropertyAddress string
    PropertyCity    string
    PropertyState   string
    OwnerAddress    string
    OwnerCity       string
    OwnerState      string
    Acres           string
    CalculatedAcres string
    SQFT            string
    Zone            string
    TaxCodes        string
    YearBuilt       string
    Appraised       string
    SaleDate        string
    SalePrice       string
    Officials       []string
    Township        string
    County          string
}

func ProcessData(parcelFile, sosFile, unifiedOutputFile, namesOutputFile string) error {
    parcelRecords, err := readParcelResults(parcelFile)
    if err != nil {
        return fmt.Errorf("error reading parcel file: %v", err)
    }

    sosRecords, err := readSOSResults(sosFile)
    if err != nil {
        return fmt.Errorf("error reading SOS file: %v", err)
    }

    mergedRecords := mergeRecords(parcelRecords, sosRecords)

    err = writeUnifiedOutput(unifiedOutputFile, mergedRecords)
    if err != nil {
        return fmt.Errorf("error writing unified output: %v", err)
    }

    err = writeNamesFile(namesOutputFile, mergedRecords)
    if err != nil {
        return fmt.Errorf("error writing names file: %v", err)
    }

    return nil
}

func readParcelResults(filename string) ([]UnifiedRecord, error) {
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

    var results []UnifiedRecord
    for _, row := range records[1:] { // Skip header
        if len(row) < len(HeadersConfig.ParcelResults) {
            continue // Skip rows with insufficient data
        }

        // Extract the first word of the property city and remove "outside"
        propertyCity := cleanCity(row[4])

        record := UnifiedRecord{
            ID:              row[0],
            PIN:             row[1],
            Name:            row[2],
            PropertyAddress: row[3],
            PropertyCity:    propertyCity,
            PropertyState:   row[5],
            OwnerAddress:    row[6],
            OwnerCity:       row[7],
            OwnerState:      row[8],
            Acres:           row[9],
            CalculatedAcres: row[10],
            SQFT:            row[11],
            Zone:            row[12],
            TaxCodes:        row[13],
            YearBuilt:       row[14],
            Appraised:       row[15],
            SaleDate:        row[16],
            SalePrice:       row[17],
            Township:        row[18],
            County:          row[19],
            BusinessName:    "",
            Officials:       []string{},
        }

        // If ID is empty, set it to the PIN
        if record.ID == "" {
            record.ID = record.PIN
        }

        results = append(results, record)
    }
    return results, nil
}

// cleanCity removes "outside" and extracts the first word of the city
func cleanCity(city string) string {
    city = strings.ToLower(city)
    // Remove "outside" and trim spaces
    city = strings.ReplaceAll(city, " outside", "")
    city = strings.TrimSpace(city)

    // Extract the first word (in case there are multiple)
    words := strings.Fields(city)
    if len(words) > 0 {
        city = words[0] // Keep the first word
    }

    // Capitalize the city name
    city = cases.Title(language.English).String(city)

    return city
}

func readSOSResults(filename string) (map[string][]string, error) {
    data, err := readCSV(filename)
    if err != nil {
        return nil, err
    }

    sosRecords := make(map[string][]string)
    for _, row := range data[1:] { // Skip header
        if len(row) >= 2 {
            businessName := row[0]
            official := row[1]
            sosRecords[businessName] = append(sosRecords[businessName], official)
        }
    }
    return sosRecords, nil
}

func mergeRecords(parcelRecords []UnifiedRecord, sosRecords map[string][]string) []UnifiedRecord {
    var mergedRecords []UnifiedRecord
    for _, record := range parcelRecords {
        if officials, ok := sosRecords[record.Name]; ok {
            for _, official := range officials {
                newRecord := record
                newRecord.BusinessName = record.Name
                newRecord.Officials = []string{official}
                mergedRecords = append(mergedRecords, newRecord)
            }
        } else {
            // For individual names, extract multiple names if present
            names := extractNames(record.Name)
            record.Officials = names
            mergedRecords = append(mergedRecords, record)
        }
    }
    return mergedRecords
}

func writeUnifiedOutput(filename string, records []UnifiedRecord) error {
    file, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    writer := csv.NewWriter(file)
    defer writer.Flush()

    // Write header
    if err := writer.Write(HeadersConfig.UnifiedResults); err != nil {
        return err
    }

    // Write data
    for _, record := range records {
        officials := record.Officials
        if len(officials) == 0 {
            officials = []string{""}
        }
        for _, official := range officials {
            title, name := extractNameAndTitle(official)
            row := []string{
                record.ID,
                record.PIN,
                record.Name,
                record.BusinessName,
                record.PropertyAddress,
                record.PropertyCity,
                record.PropertyState,
                record.OwnerAddress,
                record.OwnerCity,
                record.OwnerState,
                record.Acres,
                record.CalculatedAcres,
                record.SQFT,
                record.Zone,
                record.TaxCodes,
                record.YearBuilt,
                record.Appraised,
                record.SaleDate,
                record.SalePrice,
                record.Township,
                record.County,
                title,
                name,
            }
            if err := writer.Write(row); err != nil {
                return err
            }
        }
    }
    return nil
}

func writeNamesFile(filename string, records []UnifiedRecord) error {
    file, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    writer := csv.NewWriter(file)
    writer.Comma = ','
    defer writer.Flush()

    // Write header
    if err := writer.Write(HeadersConfig.Names); err != nil {
        return err
    }

    // Write data
    uniqueNames := make(map[string]struct{})
    for _, record := range records {
        for _, official := range record.Officials {
            _, name := extractNameAndTitle(official)
            name = strings.TrimRight(name, ",") // Remove trailing comma
            if name != "" && !strings.Contains(strings.ToLower(name), "no match") && !strings.Contains(strings.ToLower(name), "no officials found") && !isBusinessName(name) {
                key := fmt.Sprintf("%s,%s,%s", name, record.OwnerCity, record.OwnerState)
                if _, exists := uniqueNames[key]; !exists {
                    uniqueNames[key] = struct{}{}
                    if err := writer.Write([]string{name, record.OwnerCity, record.OwnerState}); err != nil {
                        return err
                    }
                }
            }
        }
    }
    return nil
}

func isBusinessName(name string) bool {
    businessIndicators := []string{
        "LLC", "INC", "CORP", "LTD", "COMPANY", "PROPERTIES", "ENTERPRISES",
        "HOLDINGS", "GROUP", "INVESTMENT", "MANAGEMENT", "ASSOCIATES",
        "SERVICES", "PROPERTY", "PARTNERS", "ASSOCIATION", "TRUST", "SOCIETY",
        "COOP", "BANK", "FEDERAL", "STATE", "DEPARTMENT", "UNIVERSITY", "ETALS",
    }
    upperName := strings.ToUpper(name)
    for _, indicator := range businessIndicators {
        if strings.Contains(upperName, indicator) {
            return true
        }
    }
    return false
}

func readCSV(filename string) ([][]string, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    reader := csv.NewReader(file)
    return reader.ReadAll()
}

func extractNames(s string) []string {
    s = strings.TrimSpace(s)
    s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")

    // Split names by common delimiters
    nameDelimiters := []string{" AND ", " & ", " ET ", ",", "/", ";", " AND/", " ET/", " AND&", " ET&", " AND-", " ET-", "&"}
    for _, delimiter := range nameDelimiters {
        if strings.Contains(strings.ToUpper(s), delimiter) {
            parts := strings.Split(s, delimiter)
            names := []string{}
            for _, part := range parts {
                part = strings.TrimSpace(part)
                part = strings.Trim(part, "&")
                part = strings.TrimSpace(part)
                cleanedName := cleanName(part)
                if cleanedName != "" {
                    names = append(names, cleanedName)
                }
            }
            return names
        }
    }

    // If no delimiter found, return the cleaned name as a single-element slice
    return []string{cleanName(s)}
}

func extractNameAndTitle(s string) (string, string) {
    s = strings.Trim(s, "\"")
    parts := strings.SplitN(s, ":", 2)
    if len(parts) == 2 {
        return strings.TrimSpace(parts[0]), cleanName(parts[1])
    }
    return "", cleanName(s)
}

func cleanName(s string) string {
    s = strings.TrimSpace(s)
    s = strings.Trim(s, "&")
    s = strings.TrimSpace(s)
    s = regexp.MustCompile(`^\b(AND|ET|&)\b\s*`).ReplaceAllString(s, "")
    s = regexp.MustCompile(`\s*\b(AND|ET|&)\b$`).ReplaceAllString(s, "")
    s = strings.TrimSpace(s)
    s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")

    // Remove unwanted substrings
    unwantedSubstrings := []string{
        " TRUSTEE", " ET AL", " SUCCESSOR", " HRS", " HEIRS", " ESTATE",
        " ETALS", " ET", " C/O",
    }
    for _, substr := range unwantedSubstrings {
        s = strings.Split(s, substr)[0]
    }
    s = strings.TrimSpace(s)

    // Remove trailing commas and periods
    s = strings.TrimRight(s, ",.")
    s = strings.ReplaceAll(s, ",", "")
    s = strings.ReplaceAll(s, ".", "")
    s = strings.TrimSpace(s)

    // Handle suffixes like Jr, Sr, III
    reSuffix := regexp.MustCompile(`(?i)\b(JR|SR|I{2,3}|IV|V|VI{1,3}|IX|X)\b$`)
    suffix := reSuffix.FindString(s)
    s = reSuffix.ReplaceAllString(s, "")
    s = strings.TrimSpace(s)

    nameParts := strings.Fields(s)

    // Remove any commas from name parts
    for i, part := range nameParts {
        nameParts[i] = strings.ReplaceAll(part, ",", "")
    }

    // Rearrangement logic
    if !isBusinessName(s) && len(nameParts) >= 2 {
        if s == strings.ToUpper(s) {
            // Name is in all uppercase letters, rearrange by moving first word to the end
            nameParts = append(nameParts[1:], nameParts[0])
        } else if len(nameParts) == 3 && len(nameParts[0]) == 1 {
            // First part is an initial, rearrange accordingly
            nameParts = append(nameParts[1:], nameParts[0])
        }
        // Else, assume the name is in correct order
    }

    s = strings.Join(nameParts, " ")

    if suffix != "" {
        s += " " + strings.ToUpper(suffix)
    }

    // Capitalize name properly
    s = cases.Title(language.English).String(strings.ToLower(s))

    // Capitalize suffixes like II, III, IV, JR, SR
    reSuffixCapital := regexp.MustCompile(`\b(Ii|Iii|Iv|V|Vi{1,3}|Ix|X|Jr|Sr)\b`)
    s = reSuffixCapital.ReplaceAllStringFunc(s, strings.ToUpper)

    return s
}

