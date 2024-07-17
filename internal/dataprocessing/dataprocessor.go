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
    Name            string
    BusinessName    string
    PropertyAddress string
    City            string
    State           string
    Acres           string
    CalculatedAcres string
    Zone            string
    TaxCodes        string
    SalePrice       string
    OwnerAddress    string
    Officials       []string
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
        if len(row) < 10 {
            continue // Skip rows with insufficient data
        }
        city, state := extractCityState(row[4]) // Extract city and state from owner address
        record := UnifiedRecord{
            ID:              row[0],
            Name:            row[2],
            BusinessName:    "", // This will be filled later if it's a business
            PropertyAddress: row[3],
            City:            city,
            State:           state,
            Acres:           row[5],
            CalculatedAcres: row[6],
            Zone:            row[7],
            TaxCodes:        row[8],
            SalePrice:       row[9],
            OwnerAddress:    row[4],
            Officials:       []string{}, // This will be filled later if applicable
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

    // Update header to match the new order
    header := []string{
        "ID", "Name", "Business Name", "Property Address", "City", "State",
        "Acres", "Calculated Acres", "Zone", "Tax Codes", "Sale Price",
        "Owner Address", "Official Title", "Official Name",
    }
    if err := writer.Write(header); err != nil {
        return err
    }

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
                record.Name,
                record.BusinessName,
                record.PropertyAddress,
                record.City,
                record.State,
                record.Acres,
                record.CalculatedAcres,
                record.Zone,
                record.TaxCodes,
                record.SalePrice,
                record.OwnerAddress,
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

    // Use a custom writer to add spaces after commas
    writer := csv.NewWriter(file)
    writer.Comma = ','
    defer writer.Flush()

    // Write header
    if err := writer.Write([]string{"Name", "City", "State"}); err != nil {
        return err
    }

    uniqueNames := make(map[string]struct{})

    for _, record := range records {
        city, state := extractCityState(record.OwnerAddress)
        for _, official := range record.Officials {
            _, name := extractNameAndTitle(official)
            name = strings.TrimRight(name, ",") // Remove trailing comma
            if name != "" && !strings.EqualFold(name, "No match") && !isBusinessName(name) {
                key := fmt.Sprintf("%s,%s,%s", name, city, state)
                if _, exists := uniqueNames[key]; !exists {
                    uniqueNames[key] = struct{}{}
                    if err := writer.Write([]string{name, city, state}); err != nil {
                        return err
                    }
                }
            }
        }
    }

    return nil
}

func isBusinessName(name string) bool {
	businessIndicators := []string{"LLC", "INC", "CORP", "LTD", "COMPANY", "PROPERTIES", "ENTERPRISES", "HOLDINGS", "GROUP", "INVESTMENT", "MANAGEMENT", "ASSOCIATES", "SERVICES", "PROPERTY"}
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

func extractCityState(address string) (string, string) {
	parts := strings.Split(address, ",")
	if len(parts) >= 3 {
		city := strings.TrimSpace(parts[len(parts)-2])
		stateZip := strings.TrimSpace(parts[len(parts)-1])
		stateParts := strings.Fields(stateZip)
		if len(stateParts) > 0 {
			return city, stateParts[0]
		}
	}
	return "", ""
}

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