package helpers

import (
	"encoding/json"
	"fmt"
	"os"
)

func Byalpha2() map[string]*Country {
	file, err := os.Open("helpers/countries.json")
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()
	var countries map[string]*Country
	decoder := json.NewDecoder(file)
	decoder.Decode(&countries)
	return countries

}

func Emoji(iso2 string) string {
	buf := [...]byte{240, 159, 135, 0, 240, 159, 135, 0}
	buf[3] = iso2[0] + (166 - 'A')
	buf[7] = iso2[1] + (166 - 'A')
	return string(buf[:])
}
