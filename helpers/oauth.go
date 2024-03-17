package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type SubscriptionPurchase struct {
	ExpiryTimeMillis    string         `json:"expiryTimeMillis"`
	LinkedPurchaseToken *string        `json:"linkedPurchaseToken"`
	Error               *ErrorResponse `json:"error"`
}
type ErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Errors  []struct {
			Message string `json:"message"`
			Domain  string `json:"domain"`
			Reason  string `json:"reason"`
		} `json:"errors"`
	} `json:"error"`
}

func ValidatePurchaseToken(token string, packageName string, productID string, retry int64) (bool, *SubscriptionPurchase) {
	b, err := os.ReadFile("credential/oauth.json")
	if err != nil {
		fmt.Printf("Unable to read client secret file: %v", err)
		return false, nil
	}

	// If modifying these scopes, delete your previously saved token.json.
	// FOR PLAY CONSOLE API
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/androidpublisher")
	if err != nil {
		fmt.Printf("Unable to parse client secret file to config: %v", err)
		return false, nil
	}
	access_token := getClient(config)
	req, err := http.NewRequest("GET", "https://androidpublisher.googleapis.com/androidpublisher/v3/applications/"+packageName+"/purchases/subscriptions/"+productID+"/tokens/"+token, nil)

	if err != nil {
		fmt.Println(err)
		return false, nil
	}

	req.Header.Add("Authorization", "Bearer "+access_token)
	req.Header.Add("Accept", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return false, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return false, nil
	}
	var response SubscriptionPurchase
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println(err)
		return false, nil
	}

	if response.Error != nil {
		fmt.Println("Error")
		fmt.Println(response)
		if retry < 0 {
			return false, &response
		}
		time.Sleep(time.Duration(retry * 1000))
		return ValidatePurchaseToken(token, packageName, productID, retry-1)

	} else {
		return true, &response
	}

}

func AckPurchase(token string, packageName string, productID string, retry int64) {
	b, err := os.ReadFile("credential/oauth.json")
	if err != nil {
		fmt.Printf("Unable to read client secret file: %v", err)
		return
	}

	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/androidpublisher")
	if err != nil {
		fmt.Printf("Unable to parse client secret file to config: %v", err)
		return
	}
	access_token := getClient(config)
	req, err := http.NewRequest("POST", "https://androidpublisher.googleapis.com/androidpublisher/v3/applications/"+packageName+"/purchases/products/"+productID+"/tokens/"+token+":acknowledge", nil)

	if err != nil {
		fmt.Println(err)
		return
	}

	req.Header.Add("Authorization", "Bearer "+access_token)
	req.Header.Add("Accept", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(bodyBytes))

	fmt.Println(resp.StatusCode)
	if resp.StatusCode != 204 {
		if retry < 0 {
			//api.SendNotification("Token: " + token + " is not able to ack. Please look up")
			return
		}
		time.Sleep(time.Duration(retry * 1000))
		AckPurchase(token, packageName, productID, retry-1)
	}

}
func getClient(config *oauth2.Config) string {
	tok, err := tokenFromDB()
	if err != nil || tok.Expiry.IsZero() || tok.Expiry.Before(time.Now()) {
		tok = refreshAccessToken(config, tok.RefreshToken)
		saveTokentoDB(tok)
	}
	return tok.AccessToken
}

// func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
// 	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
// 	fmt.Printf("Go to the following link in your browser then type the "+
// 		"authorization code: \n%v\n", authURL)

// 	var authCode string
// 	if _, err := fmt.Scan(&authCode); err != nil {
// 		log.Fatalf("Unable to read authorization code: %v", err)
// 	}

// 	tok, err := config.Exchange(context.TODO(), authCode)
// 	if err != nil {
// 		log.Fatalf("Unable to retrieve token from web: %v", err)
// 	}
// 	return tok
// }

func refreshAccessToken(config *oauth2.Config, refreshToken string) *oauth2.Token {
	ctx := context.Background()
	tok, err := config.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken}).Token()
	if err != nil {
		panic(err)
	}
	return tok
}

func tokenFromDB() (*oauth2.Token, error) {
	var token oauth2.Token
	var tokenStr string
	db.QueryRow("SELECT value FROM setting WHERE keypair = 'authToken'").Scan(&tokenStr)

	err := json.Unmarshal([]byte(tokenStr), &token)
	return &token, err

}

func saveTokentoDB(token *oauth2.Token) {
	str, err := json.Marshal(token)
	if err != nil {
		panic(err)
	}
	authToken := string(str)
	_, err = db.Query("UPDATE setting SET value = ? WHERE keypair = 'authToken'", authToken)
	if err != nil {
		panic(err)
	}

}
