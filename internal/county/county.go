package county

type Scraper interface {
    Scrape(pids []string) ([]Property, error)
}

type Property struct {
    ALPHA            string  `json:"ALPHA"`
    PIN              string  `json:"PIN"`
    NAME             string  `json:"NAME"`
    PROPERTY_ADDRESS string  `json:"PROPERTY_ADDRESS"`
    CITY             string  `json:"CITY"`
    ADDR             string  `json:"ADDR"`
    STATE            string  `json:"STATE"`
    ZIP              string  `json:"ZIP"`
    ACRES            float64 `json:"ACRES"`
    CALCACRES        float64 `json:"CALCACRES"`
    SQFT             float64 `json:"SQFT"`
    ZONE             string  `json:"ZONE"`
    TAX_CODES        string  `json:"TAX_CODES"`
    APPRAISED        float64 `json:"APPRAISED"`
    SALE_DATE        string  `json:"SALE_DATE"`
    SALE_PRICE       float64 `json:"SALE_PRICE"`
    TOWNSHIP         string  `json:"TOWNSHIP"`
    COUNTY           string  `json:"COUNTY"`
}