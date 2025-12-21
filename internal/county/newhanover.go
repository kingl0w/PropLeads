package county

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gocolly/colly/v2"
)

type NewHanoverScraper struct{}

// NewNewHanoverScraper creates a new scraper instance
func NewNewHanoverScraper() *NewHanoverScraper {
	return &NewHanoverScraper{}
}

// Scrape method to scrape multiple PIDs using the ArcGIS REST API
func (nhs *NewHanoverScraper) Scrape(pids []string) ([]Property, error) {
	var properties []Property

	for _, pid := range pids {
		log.Printf("Fetching data for PID %s from ArcGIS REST API", pid)
		// Create a new collector for each PID to avoid callback conflicts
		c := colly.NewCollector()
		info, err := nhs.getParcelInfo(c, pid)
		if err != nil {
			log.Printf("Error getting info for parcel %s: %v", pid, err)
			continue
		}
		properties = append(properties, info)
		log.Printf("Successfully retrieved data for PID %s", pid)
	}

	return properties, nil
}

// getParcelInfo retrieves parcel information from the New Hanover County ArcGIS REST API
func (nhs *NewHanoverScraper) getParcelInfo(c *colly.Collector, pid string) (Property, error) {
	// New Hanover County PropertyOwners FeatureServer endpoint
	baseURL := "https://gisport.nhcgov.com/server/rest/services/Layers/PropertyOwners/FeatureServer/0/query"
	pid = strings.TrimSpace(pid)

	var parcelInfo Property
	var err error

	c.OnResponse(func(r *colly.Response) {
		// First unmarshal into a flexible structure
		var rawResponse map[string]interface{}
		if unmarshalErr := json.Unmarshal(r.Body, &rawResponse); unmarshalErr != nil {
			err = fmt.Errorf("error parsing JSON for PID %s: %v", pid, unmarshalErr)
			log.Print(err)
			return
		}

		features, ok := rawResponse["features"].([]interface{})
		if !ok || len(features) == 0 {
			err = fmt.Errorf("no data found for parcel ID %s", pid)
			return
		}

		feature := features[0].(map[string]interface{})
		attrs := feature["attributes"].(map[string]interface{})

		// Helper function to safely get string values
		getString := func(key string) string {
			if val, ok := attrs[key]; ok && val != nil {
				return fmt.Sprintf("%v", val)
			}
			return ""
		}

		// Helper function to safely get float values
		getFloat := func(key string) float64 {
			if val, ok := attrs[key]; ok && val != nil {
				switch v := val.(type) {
				case float64:
					return v
				case string:
					f, _ := strconv.ParseFloat(v, 64)
					return f
				case int:
					return float64(v)
				}
			}
			return 0
		}

		// Helper function to safely get int values
		getInt := func(key string) int {
			if val, ok := attrs[key]; ok && val != nil {
				switch v := val.(type) {
				case float64:
					return int(v)
				case int:
					return v
				case string:
					i, _ := strconv.Atoi(v)
					return i
				}
			}
			return 0
		}

		// Build property address from components
		propertyAddress := ""
		adrno := getInt("ADRNO")
		if adrno > 0 {
			propertyAddress = strconv.Itoa(adrno)
		}
		if adradd := getString("ADRADD"); adradd != "" {
			propertyAddress += " " + strings.TrimSpace(adradd)
		}
		if adrdir := getString("ADRDIR"); adrdir != "" {
			propertyAddress += " " + strings.TrimSpace(adrdir)
		}
		if adrstr := getString("ADRSTR"); adrstr != "" {
			propertyAddress += " " + strings.TrimSpace(adrstr)
		}
		if adrsuf := getString("ADRSUF"); adrsuf != "" {
			propertyAddress += " " + strings.TrimSpace(adrsuf)
		}
		if unitno := getString("UNITNO"); unitno != "" {
			propertyAddress += " " + strings.TrimSpace(unitno)
		}
		propertyAddress = strings.TrimSpace(propertyAddress)

		// Build owner address from components
		ownerAddr1 := strings.TrimSpace(getString("OWNER_ADDR1"))
		ownerAddr2 := strings.TrimSpace(getString("OWNER_ADDR2"))
		ownerAddress := ownerAddr1
		if ownerAddr2 != "" && ownerAddress != "" {
			ownerAddress += ", " + ownerAddr2
		} else if ownerAddr2 != "" {
			ownerAddress = ownerAddr2
		}

		parcelInfo = Property{
			ALPHA:            pid,
			PIN:              getString("PARID"),
			NAME:             strings.TrimSpace(getString("OWN1")),
			PROPERTY_ADDRESS: propertyAddress,
			PROPERTY_CITY:    strings.TrimSpace(getString("CITYNAME")),
			PROPERTY_STATE:   "NC",
			OWNER_ADDRESS:    ownerAddress,
			OWNER_CITY:       strings.TrimSpace(getString("OWNER_CITY")),
			OWNER_STATE:      strings.TrimSpace(getString("OWNER_STATE")),
			OWNER_ZIP:        strings.TrimSpace(getString("OWNER_ZIP")),
			ACRES:            getFloat("ACRES"),
			CALCACRES:        0, // Not available in this API
			SQFT:             getFloat("SFLA"),
			ZONE:             strings.TrimSpace(getString("ZONING")),
			TAX_CODES:        "", // Not directly available
			YEAR_BUILT:       "", // Not available in PropertyOwners layer
			APPRAISED:        getFloat("APRTOT"),
			SALE_DATE:        strings.TrimSpace(getString("SALE_DATE")),
			SALE_PRICE:       getFloat("SALE_PRICE"),
			TOWNSHIP:         "", // Not available
			COUNTY:           "New Hanover",
		}

		log.Printf("Parsed data for PID %s: Owner=%s, Address=%s, City=%s, Acres=%.2f, Zone=%s",
			pid, parcelInfo.NAME, parcelInfo.PROPERTY_ADDRESS, parcelInfo.PROPERTY_CITY,
			parcelInfo.ACRES, parcelInfo.ZONE)
	})

	c.OnError(func(r *colly.Response, e error) {
		err = fmt.Errorf("request failed for PID %s: %v", pid, e)
	})

	// Query by PID field - the API uses PARID field
	url := fmt.Sprintf("%s?f=json&where=PARID='%s'&returnGeometry=false&outFields=*", baseURL, pid)
	log.Printf("Querying: %s", url)

	err = c.Visit(url)
	if err != nil {
		return Property{}, fmt.Errorf("failed to visit URL for PID %s: %v", pid, err)
	}

	return parcelInfo, err
}

// NewHanoverResponse represents the API response structure
type NewHanoverResponse struct {
	Features []struct {
		Attributes struct {
			PARID        string  `json:"PARID"`
			OWN1         string  `json:"OWN1"`
			OWNER_ADDR1  string  `json:"OWNER_ADDR1"`
			OWNER_ADDR2  string  `json:"OWNER_ADDR2"`
			OWNER_CITY   string  `json:"OWNER_CITY"`
			OWNER_STATE  string  `json:"OWNER_STATE"`
			OWNER_ZIP    string  `json:"OWNER_ZIP"`
			ADRNO        int     `json:"ADRNO"`
			ADRADD       string  `json:"ADRADD"`
			UNITNO       string  `json:"UNITNO"`
			ADRSTR       string  `json:"ADRSTR"`
			ADRSUF       string  `json:"ADRSUF"`
			ADRDIR       string  `json:"ADRDIR"`
			CITYNAME     string  `json:"CITYNAME"`
			ACRES        float64 `json:"ACRES"`
			ZONING       string  `json:"ZONING"`
			APRLAND      float64 `json:"APRLAND"`
			APRBLDG      float64 `json:"APRBLDG"`
			APRTOT       float64 `json:"APRTOT"`
			SALE_DATE    string  `json:"SALE_DATE"`
			SALE_PRICE   float64 `json:"SALE_PRICE"`
			SFLA         float64 `json:"SFLA"`
			AREASUM      float64 `json:"AREASUM"`
		} `json:"attributes"`
	} `json:"features"`
}
