package geocoder

type GoogleResponse struct {
	Results []*ResultSet         `json:"results"`
	Status  GoogleResponseStatus `json:"status"`
}

type ResultSet struct {
	AddressComponents []AddressComponent `json:"address_components"`
	FormattedAddress  string             `json:"formatted_address"`
	Geometry          Geometry           `json:"geometry"`
	PlaceID           string             `json:"place_id"`
	Types             []string           `json:"types"`
}

type AddressComponent struct {
	LongName  string   `json:"long_name"`
	ShortName string   `json:"short_name"`
	Types     []string `json:"types"`
}

type Geometry struct {
	Location     Coordinate `json:"location"`
	LocationType string     `json:"location_type"`
}

type Coordinate struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type Bounds struct {
	SouthWest Coordinate `json:"southwest"`
	NorthEast Coordinate `json:"northeast"`
}

type BusinessKey struct {
	ClientID   string
	SigningKey string
	Channel    string
}

type GoogleResponseStatus string

const (
	GRS_ZERO_RESULTS     GoogleResponseStatus = "ZERO_RESULTS"
	GRS_REQUEST_DENIED   GoogleResponseStatus = "REQUEST_DENIED"
	GRS_INVALID_REQUEST  GoogleResponseStatus = "INVALID_REQUEST"
	GRS_UNKNOWN_ERROR    GoogleResponseStatus = "UNKNOWN_ERROR"
	GRS_OVER_QUERY_LIMIT GoogleResponseStatus = "OVER_QUERY_LIMIT"
	GRS_OK               GoogleResponseStatus = "OK"
)
