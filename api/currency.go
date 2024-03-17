package api

import (
	"binchecker/credential"
	"encoding/json"
	"net/http"
)

type Currency struct {
	Rates map[string]float64 `json:"rates"`
}

func GetPriceByCurrency(to string) (float64, error) {
	response, err := http.Get("http://data.fixer.io/api/latest?access_key=" + credential.FIXER_API_KEY)
	if err != nil {
		return 0, err
	}
	var result Currency
	json.NewDecoder(response.Body).Decode(&result)
	usd := result.Rates["USD"]
	curr := result.Rates[to]
	return (curr / usd), nil
}
