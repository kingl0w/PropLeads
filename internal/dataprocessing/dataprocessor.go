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

    wpOutputFile := strings.Replace(namesOutputFile, "names.csv", "names_for_whitepages.csv", 1)
    err = writeWhitePagesFormat(wpOutputFile, mergedRecords)
    if err != nil {
        return fmt.Errorf("error writing WhitePages format file: %v", err)
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
    for _, row := range records[1:] {
        if len(row) < len(HeadersConfig.ParcelResults) {
            continue
        }

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

        if record.ID == "" {
            record.ID = record.PIN
        }

        results = append(results, record)
    }
    return results, nil
}

func cleanCity(city string) string {
    city = strings.ToLower(city)
    city = strings.ReplaceAll(city, " outside", "")
    city = strings.TrimSpace(city)

    words := strings.Fields(city)
    if len(words) > 0 {
        city = words[0]
    }

    city = cases.Title(language.English).String(city)

    return city
}

func readSOSResults(filename string) (map[string][]string, error) {
    data, err := readCSV(filename)
    if err != nil {
        return nil, err
    }

    sosRecords := make(map[string][]string)
    for _, row := range data[1:] {
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

    if err := writer.Write(HeadersConfig.UnifiedResults); err != nil {
        return err
    }

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

    if err := writer.Write(HeadersConfig.Names); err != nil {
        return err
    }

    uniqueNames := make(map[string]struct{})
    for _, record := range records {
        for _, official := range record.Officials {
            _, name := extractNameAndTitle(official)
            name = strings.TrimRight(name, ",")
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

//whitepages requires no header row
func writeWhitePagesFormat(filename string, records []UnifiedRecord) error {
    file, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    writer := csv.NewWriter(file)
    writer.Comma = ','
    defer writer.Flush()

    uniqueNames := make(map[string]struct{})
    for _, record := range records {
        for _, official := range record.Officials {
            _, name := extractNameAndTitle(official)
            name = strings.TrimRight(name, ",")
            if name != "" && !strings.Contains(strings.ToLower(name), "no match") && !strings.Contains(strings.ToLower(name), "no officials found") && !isBusinessName(name) {
                key := fmt.Sprintf("%s,%s,%s", name, record.OwnerCity, record.OwnerState)
                if _, exists := uniqueNames[key]; !exists {
                    uniqueNames[key] = struct{}{}

                    firstName, lastName := splitName(name)

                    if err := writer.Write([]string{firstName, lastName, record.OwnerCity, record.OwnerState}); err != nil {
                        return err
                    }
                }
            }
        }
    }
    return nil
}

//nc property records use "last first middle" format
func splitName(fullName string) (string, string) {
    fullName = strings.TrimSpace(fullName)

    if fullName == "" {
        return "", ""
    }

    parts := strings.Fields(fullName)

    if len(parts) == 0 {
        return "", ""
    } else if len(parts) == 1 {
        return "", parts[0]
    } else if len(parts) == 2 {
        return parts[1], parts[0]
    } else {
        lastName := parts[0]
        firstName := strings.Join(parts[1:], " ")
        return firstName, lastName
    }
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

    unwantedSubstrings := []string{" TRUSTEE", " ET AL", " SUCCESSOR", " HRS", " HEIRS", " ESTATE", " ETALS", " ET", " C/O"}
    for _, substr := range unwantedSubstrings {
        s = strings.Split(s, substr)[0]
    }
    s = strings.TrimSpace(s)

    s = strings.TrimRight(s, ",.")

    reSuffix := regexp.MustCompile(`(?i)\s+(I{1,3}|IV|V|VI{1,3}|JR|SR)\.?$`)
    suffix := reSuffix.FindString(s)
    s = reSuffix.ReplaceAllString(s, "")
    
    nameParts := strings.Fields(s)

    corrections := map[string]string{
        "Skippeer Morris Ray": "Skipper Morris Ray",
    }
    if correctedName, exists := corrections[s]; exists {
        s = correctedName
    }

    if !isBusinessName(s) && len(nameParts) >= 2 {
        if strings.Contains(nameParts[0], ",") {
            lastName := strings.TrimRight(nameParts[0], ",")
            firstNames := nameParts[1:]
            s = strings.Join(firstNames, " ") + " " + lastName
        } else {
            s = strings.Join(nameParts, " ")
        }
    } else {
        s = strings.Join(nameParts, " ")
    }

    s = strings.TrimSpace(s)
    if suffix != "" {
        s += " " + strings.TrimSpace(suffix)
    }

    s = cases.Title(language.English).String(strings.ToLower(s))

    reRoman := regexp.MustCompile(`\b(Ii|Iii|Iv|V|Vi|Vii|Viii|Ix|X)\b`)
    s = reRoman.ReplaceAllStringFunc(s, strings.ToUpper)

    return s
}
