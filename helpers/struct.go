package helpers

type CardType struct {
	Valid       bool
	Network     string
	NiceNetwork string
	Gaps        []int32
	Lengths     []int32
	CodeName    string
	CodeSize    int32
}

type Ip struct {
	Ip      int64
	Country string
	State   string
	City    string
}

type Country struct {
	Name             string              `json:"name"`
	Alpha2           string              `json:"alpha2"`
	Alpha3           string              `json:"alpha3"`
	Capital          string              `json:"capital"`
	Domain           string              `json:"domain"`
	Emoji            string              `json:"emoji"`
	Latitude         float32             `json:"latitude"`
	Longitude        float32             `json:"longitude"`
	ContinentCode    string              `json:"continentcode"`
	Region           string              `json:"region"`
	SubRegion        string              `json:"subregion"`
	Language         string              `json:"language"`       //deprecated
	LanguageAlpha2   string              `json:"languagealpha2"` //deprecated
	LanguageAlpha3   string              `json:"languagealpha3"` //deprecated
	CurrencySymbol   string              `json:"currencySymbol"` //deprecated
	CurrencyCode     string              `json:"currencyCode"`   //deprecated
	CurrencyName     string              `json:"currencyName"`   //deprecated
	Developed        bool                `json:"developed"`
	CallingCode      int32               `json:"callingCode"`
	Numeric          int32               `json:"numeric"`
	StartofWeek      string              `json:"startofWeek"`
	Languages        map[string]Language `json:"languages"`
	Currencies       map[string]Currency `json:"currencies"`
	PostalCodeFormat string              `json:"postalCodeFormat" default:""`
	Timezones        []string            `json:"timezones"`
}

type Language struct {
	Name   string `json:"name"`
	Alpha2 string `json:"alpha2"`
	Alpha3 string `json:"alpha3"`
}

type Currency struct {
	Name   string `json:"name"`
	Code   string `json:"code"`
	Symbol string `json:"symbol"`
}
type CapchaResponse struct {
	Success     bool     `json:"success"`
	Score       float64  `json:"score"`
	Action      string   `json:"action"`
	ChallengeTs string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes"`
}
