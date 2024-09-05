package county

type Property struct {
    ALPHA            string  `json:"ALPHA"`
    PIN              string  `json:"PIN"`
    NAME             string  `json:"NAME"`
    PROPERTY_ADDRESS string  `json:"PROPERTY_ADDRESS"`
    PROPERTY_CITY    string  `json:"PROPERTY_CITY"`
    PROPERTY_STATE   string  `json:"PROPERTY_STATE"`
    OWNER_ADDRESS    string  `json:"OWNER_ADDRESS"`
    OWNER_CITY       string  `json:"OWNER_CITY"`
    OWNER_STATE      string  `json:"OWNER_STATE"`
    OWNER_ZIP        string  `json:"OWNER_ZIP"`
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

type Scraper interface {
    Scrape(pids []string) ([]Property, error)
}