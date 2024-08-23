package csv

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/kingl0w/PropLeads/internal/county"
	"github.com/kingl0w/PropLeads/internal/sos"
)

func WriteParcelResults(filename string, parcels []county.Property) error {
    file, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    writer := csv.NewWriter(file)
    defer writer.Flush()

    // Write header
    header := []string{"ID", "PIN", "Owner", "Property Address", "City", "Owner Address", "Acres", "Calculated Acres", "SQFT", "Zone", "Tax Codes", "Appraised", "Sale Date", "Sale Price", "Township", "County"}
    if err := writer.Write(header); err != nil {
        return err
    }

    // Write data
    for _, parcel := range parcels {
        ownerAddress := fmt.Sprintf("%s, %s, %s %s", parcel.ADDR, parcel.CITY, parcel.STATE, parcel.ZIP)
        
        row := []string{
            parcel.ALPHA,
            parcel.PIN,
            parcel.NAME,
            parcel.PROPERTY_ADDRESS,
            parcel.CITY,
            ownerAddress,
            formatFloat(parcel.ACRES),
            formatFloat(parcel.CALCACRES),
            formatFloat(parcel.SQFT),
            parcel.ZONE,
            parcel.TAX_CODES,
            formatFloat(parcel.APPRAISED),
            parcel.SALE_DATE,
            formatFloat(parcel.SALE_PRICE),
            parcel.TOWNSHIP,
            parcel.COUNTY,
        }
        if err := writer.Write(row); err != nil {
            return err
        }
    }
    return nil
}

func formatFloat(f float64) string {
    if f == 0 {
        return ""
    }
    return fmt.Sprintf("%.2f", f)
}

func WriteSOSResults(filename string, businesses []sos.BusinessInfo) error {
    file, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    writer := csv.NewWriter(file)
    defer writer.Flush()

    header := []string{"Business Name", "Company Officials"}
    if err := writer.Write(header); err != nil {
        return err
    }

    for _, business := range businesses {
        if len(business.CompanyOfficials) == 1 && business.CompanyOfficials[0].Name == "No match" {
            row := []string{
                business.BusinessName,
                "No match",
            }
            if err := writer.Write(row); err != nil {
                return err
            }
        } else {
            for _, official := range business.CompanyOfficials {
                officialInfo := strings.TrimSpace(official.Title + ": " + official.Name)
                officialInfo = strings.Join(strings.Fields(officialInfo), " ") // Remove extra spaces
                row := []string{
                    business.BusinessName,
                    officialInfo,
                }
                if err := writer.Write(row); err != nil {
                    return err
                }
            }
        }
    }

    return nil
}