package api

type IPInfo struct {
	CountryCode string `json:"country"`
	IP          int64  `json:"ip"`
}
