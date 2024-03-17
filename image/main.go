package main

import (
	"binchecker/credential"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Data struct {
	Photos []struct {
		URL string `json:"url"`
		Src struct {
			Original string `json:"original"`
		} `json:"src"`
	} `json:"photos"`
}

func randInt(max int) int {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	return r.Intn(max)
}

// https://www.pexels.com/photo/blue-pills-12785272/
func main() {
	color := []string{"white"}
	topics := []string{"summer"}
	requsturl := "https://api.pexels.com/v1/search?query=" + topics[randInt(len(topics))] + "&color=" + color[randInt(len(color))] + "&page=" + strconv.Itoa(randInt(4500/2)) + "&per_page=1"
	client := &http.Client{}
	req, _ := http.NewRequest("GET", requsturl, nil)
	req.Header.Add("Authorization", credential.PEXELS_API_KEY)
	response, err := client.Do(req)
	if err != nil {
		fmt.Println(err.Error())

	}
	responseData, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Println(err)

	}

	var responseObject Data
	json.Unmarshal(responseData, &responseObject)

	out, _ := os.Create("default.jpg")
	defer out.Close()
	if len(responseObject.Photos) == 0 {
		fmt.Println("No Images. Try Again")
		return
	}
	resp, err := http.Get(responseObject.Photos[0].Src.Original + "?cs=srgb&fm=jpg&h=600&w=1050&fit=crop&dl=default.jpg")
	if err != nil {
		fmt.Println(err.Error())
	}
	defer resp.Body.Close()
	io.Copy(out, resp.Body)

	f, _ := os.Create("image.txt")
	f.WriteString(responseObject.Photos[0].URL)
	f.Close()

	fmt.Println(requsturl)
	fmt.Println("done")
}
