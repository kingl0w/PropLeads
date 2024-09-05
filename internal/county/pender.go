package county

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gocolly/colly/v2"
)

type PenderScraper struct{}

func NewPenderScraper() *PenderScraper {
    return &PenderScraper{}
}

func (p *PenderScraper) Scrape(pids []string) ([]Property, error) {
    c := colly.NewCollector()
    var properties []Property
    for _, parcelID := range pids {
        info, err := getParcelInfo(c, parcelID)
        if err != nil {
            fmt.Printf("Error getting info for parcel %s: %v\n", parcelID, err)
            continue
        }
        properties = append(properties, info)
    }
    return properties, nil
}

func getParcelInfo(c *colly.Collector, parcelID string) (Property, error) {
    baseURL := "https://gis.pendercountync.gov/arcgis/rest/services/Layers/MapServer/4/query"
    var parcelInfo Property
    var err error
    c.OnResponse(func(r *colly.Response) {
        var response Response
        err = json.Unmarshal(r.Body, &response)
        if err != nil {
            return
        }
        if len(response.Features) > 0 {
            attrs := response.Features[0].Attributes
            parcelInfo = Property{
                ALPHA:            attrs.ALPHA,
                PIN:              attrs.PIN,
                NAME:             attrs.NAME,
                PROPERTY_ADDRESS: attrs.PROPERTY_ADDRESS,
                PROPERTY_CITY:    "", // Leave property city blank for Pender County
                PROPERTY_STATE:   "NC", // Always set to NC for Pender County
                OWNER_ADDRESS:    attrs.ADDR,
                OWNER_CITY:       attrs.CITY, // Use the CITY field from the API response
                OWNER_STATE:      attrs.STATE,
                OWNER_ZIP:        attrs.ZIP,
                ACRES:            attrs.ACRES,
                CALCACRES:        attrs.CALCACRES,
                SQFT:             0, // Set to 0 if not available
                ZONE:             attrs.ZONE,
                TAX_CODES:        attrs.TAX_CODES,
                APPRAISED:        0, // Set to 0 if not available
                SALE_DATE:        "", // Set to empty string if not available
                SALE_PRICE:       attrs.SALE_PRICE,
                TOWNSHIP:         attrs.TNSH_DESC,
                COUNTY:           "Pender",
            }
        } else {
            err = fmt.Errorf("no data found for parcel ID %s", parcelID)
        }
    })
    url := fmt.Sprintf("%s?f=json&where=ALPHA='%s'&returnGeometry=false&outFields=*", baseURL, parcelID)
    err = c.Visit(url)
    if err != nil {
        return Property{}, err
    }
    return parcelInfo, err
}
func extractCityStateZip(address string) (string, string, string) {
    parts := strings.Split(address, ",")
    if len(parts) >= 2 {
        stateZip := strings.TrimSpace(parts[len(parts)-1])
        stateParts := strings.Fields(stateZip)
        if len(stateParts) >= 2 {
            return "", stateParts[0], stateParts[1]
        }
        if len(stateParts) == 1 {
            return "", "NC", stateParts[0] // Assume NC if only ZIP is present
        }
    }
    return "", "NC", "" // Default to NC if unable to parse
}

type Response struct {
    Features []struct {
        Attributes struct {
            ALPHA            string  `json:"ALPHA"`
            PIN              string  `json:"PIN"`
            CALCACRES        float64 `json:"CALCACRES"`
            NAME             string  `json:"NAME"`
            ADDR             string  `json:"ADDR"`
            CITY             string  `json:"CITY"`
            STATE            string  `json:"STATE"`
            ZIP              string  `json:"ZIP"`
            PROPERTY_ADDRESS string  `json:"PROPERTY_ADDRESS"`
            ACRES            float64 `json:"ACRES"`
            ZONE             string  `json:"ZONE"`
            TAX_CODES        string  `json:"TAX_CODES"`
            SALE_PRICE       float64 `json:"SALE_PRICE"`
            TNSH_DESC        string  `json:"TNSH_DESC"`
        } `json:"attributes"`
    } `json:"features"`
}