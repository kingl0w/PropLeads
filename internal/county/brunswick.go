package county

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gocolly/colly/v2"
)

type BrunswickScraper struct{}

func NewBrunswickScraper() *BrunswickScraper {
	return &BrunswickScraper{}
}

func (bs *BrunswickScraper) Scrape(pids []string) ([]Property, error) {
	var properties []Property

	for _, pid := range pids {
		log.Printf("Fetching data for PID %s from ArcGIS REST API", pid)
		//new collector per pid to avoid callback conflicts
		c := colly.NewCollector()
		info, err := bs.getParcelInfo(c, pid)
		if err != nil {
			log.Printf("Error getting info for parcel %s: %v", pid, err)
			continue
		}
		properties = append(properties, info)
		log.Printf("Successfully retrieved data for PID %s", pid)
	}

	return properties, nil
}

func (bs *BrunswickScraper) getParcelInfo(c *colly.Collector, pid string) (Property, error) {
	baseURL := "https://bcgis.brunswickcountync.gov/arcgis/rest/services/Mapping/DataViewerLive/MapServer/26/query"
	pid = strings.TrimSpace(pid)

	var parcelInfo Property
	var err error

	c.OnResponse(func(r *colly.Response) {
		var response BrunswickResponse
		err = json.Unmarshal(r.Body, &response)
		if err != nil {
			log.Printf("Error parsing JSON for PID %s: %v", pid, err)
			return
		}

		if len(response.Features) > 0 {
			attrs := response.Features[0].Attributes

			propertyAddress := strings.TrimSpace(attrs.HouseNumber)
			if attrs.StreetName != "" {
				propertyAddress += " " + strings.TrimSpace(attrs.StreetName)
			}
			if attrs.StreetType != "" {
				propertyAddress += " " + strings.TrimSpace(attrs.StreetType)
			}
			if attrs.StreetDirection != "" {
				propertyAddress += " " + strings.TrimSpace(attrs.StreetDirection)
			}

			yearBuilt := ""
			if attrs.ActualYearBuilt > 0 {
				yearBuilt = strconv.Itoa(attrs.ActualYearBuilt)
			}

			acres := 0.0
			if attrs.DeedAcreage != "" {
				acres, _ = strconv.ParseFloat(strings.TrimSpace(attrs.DeedAcreage), 64)
			}

			ownerAddress := strings.TrimSpace(attrs.Address1)
			if attrs.Address2 != "" && ownerAddress != "" {
				ownerAddress += ", " + strings.TrimSpace(attrs.Address2)
			} else if attrs.Address2 != "" {
				ownerAddress = strings.TrimSpace(attrs.Address2)
			}

			parcelInfo = Property{
				ALPHA:            pid,
				PIN:              strings.TrimSpace(attrs.PIN),
				NAME:             strings.TrimSpace(attrs.Name1),
				PROPERTY_ADDRESS: propertyAddress,
				PROPERTY_CITY:    "",
				PROPERTY_STATE:   "NC",
				OWNER_ADDRESS:    ownerAddress,
				OWNER_CITY:       strings.TrimSpace(attrs.City),
				OWNER_STATE:      strings.TrimSpace(attrs.State),
				OWNER_ZIP:        strings.TrimSpace(attrs.ZipCode),
				ACRES:            acres,
				CALCACRES:        attrs.CALCAC,
				SQFT:             attrs.TotalActualAreaHeated,
				ZONE:             strings.TrimSpace(attrs.Zoning),
				TAX_CODES:        "",
				YEAR_BUILT:       yearBuilt,
				APPRAISED:        0,
				SALE_DATE:        strings.TrimSpace(attrs.DeedDate),
				SALE_PRICE:       0,
				TOWNSHIP:         "",
				COUNTY:           "Brunswick",
			}

			log.Printf("Parsed data for PID %s: Owner=%s, Address=%s, Acres=%.2f, Zone=%s",
				pid, parcelInfo.NAME, parcelInfo.PROPERTY_ADDRESS, parcelInfo.ACRES, parcelInfo.ZONE)
		} else {
			err = fmt.Errorf("no data found for parcel ID %s", pid)
		}
	})

	c.OnError(func(r *colly.Response, e error) {
		err = fmt.Errorf("request failed for PID %s: %v", pid, e)
	})

	url := fmt.Sprintf("%s?f=json&where=ParcelNumber='%s'&returnGeometry=false&outFields=*", baseURL, pid)
	log.Printf("Querying: %s", url)

	err = c.Visit(url)
	if err != nil {
		return Property{}, fmt.Errorf("failed to visit URL for PID %s: %v", pid, err)
	}

	return parcelInfo, err
}

type BrunswickResponse struct {
	Features []struct {
		Attributes struct {
			ParcelNumber           string  `json:"ParcelNumber"`
			PIN                    string  `json:"PIN"`
			CALCAC                 float64 `json:"CALCAC"`
			DeedAcreage            string  `json:"DeedAcreage"`
			LegalDescription       string  `json:"LegalDescription"`
			Name1                  string  `json:"Name1"`
			Name2                  string  `json:"Name2"`
			Address1               string  `json:"Address1"`
			Address2               string  `json:"Address2"`
			City                   string  `json:"City"`
			State                  string  `json:"State"`
			ZipCode                string  `json:"ZipCode"`
			HouseNumber            string  `json:"HouseNumber"`
			StreetName             string  `json:"StreetName"`
			StreetType             string  `json:"StreetType"`
			StreetDirection        string  `json:"StreetDirection"`
			Zoning                 string  `json:"Zoning"`
			ActualYearBuilt        int     `json:"ActualYearBuilt"`
			TotalActualAreaHeated  float64 `json:"TotalAcutalAreaHeated"` //api has typo "Acutal" - do not fix
			HeatedAreaCard         float64 `json:"HeatedAreaCard"`
			DeedDate               string  `json:"DeedDate"`
			DeedBook               string  `json:"DeedBook"`
			DeedPage               string  `json:"DeedPage"`
		} `json:"attributes"`
	} `json:"features"`
}
