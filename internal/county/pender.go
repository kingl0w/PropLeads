package county

import (
	"encoding/json"
	"fmt"

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
            parcelInfo = response.Features[0].Attributes
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

type Response struct {
    Features []struct {
        Attributes Property `json:"attributes"`
    } `json:"features"`
}