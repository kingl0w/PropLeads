package dataprocessing

var HeadersConfig = struct {
    ParcelResults  []string
    SOSResults     []string
    UnifiedResults []string
    Names          []string
}{
    ParcelResults: []string{
        "ID", "PIN", "Owner", "Property Address", "Property City", "Property State",
        "Owner Address", "Owner City", "Owner State", "Acres", "Calculated Acres",
        "SQFT", "Zone", "Tax Codes", "Year Built", "Appraised", "Sale Date", "Sale Price",
        "Township", "County",
    },
    SOSResults: []string{
        "Business Name", "Company Officials",
    },
    UnifiedResults: []string{
        "ID", "PIN", "Owner", "Business Name", "Property Address", "Property City", "Property State",
        "Owner Address", "Owner City", "Owner State", "Acres", "Calculated Acres",
        "SQFT", "Zone", "Tax Codes", "Year Built", "Appraised", "Sale Date", "Sale Price",
        "Township", "County", "Official Title", "Official Name",
    },
    Names: []string{
        "Name", "City", "State",
    },
}