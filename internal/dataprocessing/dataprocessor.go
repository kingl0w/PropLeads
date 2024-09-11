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
        record := UnifiedRecord{
            ID:              row[0],
            PIN:             row[1],
            Name:            row[2],
            PropertyAddress: row[3],
            PropertyCity:    row[4],
            PropertyState:   row[5],
            OwnerAddress:    row[6],
            OwnerCity:       row[7],
            OwnerState:      row[8],
            Acres:           row[9],
            CalculatedAcres: row[10],
            SQFT:            row[11],
            Zone:            row[12],
            TaxCodes:        row[13],
            Appraised:       row[14],
            SaleDate:        row[15],
            SalePrice:       row[16],
            Township:        row[17],
            County:          row[18],
            BusinessName:    "",
            Officials:       []string{},
        }

        results = append(results, record)
    }
    return results, nil
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
            record.Officials = []string{extractName(record.Name)}
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
            parts := strings.SplitN(official, ":", 2)
            title := ""
            name := official
            if len(parts) == 2 {
                title = strings.TrimSpace(parts[0])
                name = strings.TrimSpace(parts[1])
            }
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
            if name != "" && !strings.EqualFold(name, "No match") && !isBusinessName(name) {
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
	businessIndicators := []string{"LLC", "INC", "CORP", "LTD", "COMPANY", "PROPERTIES", "ENTERPRISES", "HOLDINGS", "GROUP", "INVESTMENT", "MANAGEMENT", "ASSOCIATES", "SERVICES", "PROPERTY", "PARTNERS", "ASSOCIATION", "TRUST"}
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

// func extractCityState(address string) (string, string) {
// 	parts := strings.Split(address, ",")
// 	if len(parts) >= 3 {
// 		city := strings.TrimSpace(parts[len(parts)-2])
// 		stateZip := strings.TrimSpace(parts[len(parts)-1])
// 		stateParts := strings.Fields(stateZip)
// 		if len(stateParts) > 0 {
// 			return city, stateParts[0]
// 		}
// 	}
// 	return "", ""
// }

func extractName(s string) string {
	s = strings.Trim(s, "\"")

	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 {
		s = strings.TrimSpace(parts[1])
	}

	s = strings.Split(s, " TRUSTEE")[0]
	s = strings.Split(s, " ET AL")[0]
    s = strings.Split(s, " SUCCESSOR")[0]

	s = strings.TrimSpace(s)

	re := regexp.MustCompile(`(?i)\s+(I{1,3}|IV|V|VI{1,3}|JR|SR)\.?$`)
	suffix := re.FindString(s)
	s = re.ReplaceAllString(s, "")

	nameParts := strings.Fields(s)
	if len(nameParts) > 0 && strings.Contains(nameParts[0], ",") {
		lastName := strings.TrimRight(nameParts[0], ",")
		firstName := strings.Join(nameParts[1:], " ")
		s = firstName + " " + lastName
	}

	unwantedSuffixes := []string{"CBE", "ET AL"}
	for _, unwanted := range unwantedSuffixes {
		s = strings.Replace(s, unwanted, "", -1)
	}

	s = strings.TrimSpace(s)
	if suffix != "" {
		s += " " + strings.TrimSpace(suffix)
	}

	caser := cases.Title(language.English)
	s = caser.String(strings.ToLower(s))

	re = regexp.MustCompile(`\b(Ii|Iii|Iv|Vi|Vii|Viii|Ix|X)\b`)
	s = re.ReplaceAllStringFunc(s, strings.ToUpper)

	return s
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
	s = strings.Split(s, " TRUSTEE")[0]
	s = strings.Split(s, " ET AL")[0]
	s = strings.TrimSpace(s)
    s = strings.TrimRight(s, ",")

	re := regexp.MustCompile(`(?i)\s+(I{1,3}|IV|V|VI{1,3}|JR|SR)\.?$`)
	suffix := re.FindString(s)
	s = re.ReplaceAllString(s, "")

	nameParts := strings.Fields(s)
	if len(nameParts) > 0 && strings.Contains(nameParts[0], ",") {
		lastName := strings.TrimRight(nameParts[0], ",")
		firstName := strings.Join(nameParts[1:], " ")
		s = firstName + " " + lastName
	}

	unwantedSuffixes := []string{"CBE", "ET AL"}
	for _, unwanted := range unwantedSuffixes {
		s = strings.Replace(s, unwanted, "", -1)
	}

	s = strings.TrimSpace(s)
	if suffix != "" {
		s += " " + strings.TrimSpace(suffix)
	}

	caser := cases.Title(language.English)
	s = caser.String(strings.ToLower(s))

	re = regexp.MustCompile(`\b(Ii|Iii|Iv|Vi|Vii|Viii|Ix|X)\b`)
	s = re.ReplaceAllStringFunc(s, strings.ToUpper)

	return s
}