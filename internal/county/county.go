package county

type Scraper interface {
    Scrape(pids []string) ([]Property, error)
}

type Property struct {
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
    COUNTY           string  `json:"COUNTY"` 
    TOWNSHIP         string  `json:"TOWNSHIP"`
}