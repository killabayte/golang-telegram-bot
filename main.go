package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// urlEncode performs URL encoding similar to Java's URLEncoder.encode but replaces '+' with '%20'.
func urlEncode(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// getRequestParamString constructs a sorted parameter string from the request parameters.
func getRequestParamString(params map[string]string) string {
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var paramStrBuilder strings.Builder
	for _, k := range keys {
		paramStrBuilder.WriteString(fmt.Sprintf("%s=%s&", k, urlEncode(params[k])))
	}
	paramStr := paramStrBuilder.String()
	return strings.TrimSuffix(paramStr, "&")
}

// sign generates the signature for the request.
func sign(accessKey, secretKey, reqTime, paramStr string) string {
	toSign := accessKey + reqTime + paramStr

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(toSign))
	return hex.EncodeToString(mac.Sum(nil))
}

type OpenPositionsResponse struct {
	Data []struct {
		Symbol       string  `json:"symbol"`
		HoldAvgPrice float64 `json:"holdAvgPrice"` // Changed from string to float64
	} `json:"data"`
}

type FairPriceResponse struct {
	Data struct {
		FairPrice float64 `json:"fairPrice"`
	} `json:"data"`
}

func queryFairPriceForSymbol(client *http.Client, accessKey, secretKey, baseURL, symbol string, holdAvgPrice float64) {
	reqTime := strconv.FormatInt(time.Now().Unix()*1000, 10)
	endpoint := fmt.Sprintf("/api/v1/contract/fair_price/%s", symbol)
	signature := sign(accessKey, secretKey, reqTime, "")

	fullURL := baseURL + endpoint

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		fmt.Printf("Error creating request for symbol %s: %v\n", symbol, err)
		return
	}

	req.Header.Add("ApiKey", accessKey)
	req.Header.Add("Request-Time", reqTime)
	req.Header.Add("Signature", signature)
	req.Header.Add("Content-Type", "application/json")

	response, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending request for symbol %s: %v\n", symbol, err)
		return
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Printf("Error reading response body for symbol %s: %v\n", symbol, err)
		return
	}

	var fairPriceResp FairPriceResponse
	if err := json.Unmarshal(body, &fairPriceResp); err != nil {
		fmt.Printf("Error decoding fair price response for %s: %v\n", symbol, err)
		return
	}

	// Direct comparison, since both are now float64
	if fairPriceResp.Data.FairPrice != holdAvgPrice {
		difference := fairPriceResp.Data.FairPrice - holdAvgPrice
		percentageDifference := (difference / holdAvgPrice) * 100 // Calculate percentage difference

		if difference > 0 {
			fmt.Printf("For %s, FairPrice (%f) is greater than HoldAvgPrice (%f) by: %f (%.2f%%)\n", symbol, fairPriceResp.Data.FairPrice, holdAvgPrice, difference, percentageDifference)
		} else {
			// Note: difference is negative here, so we multiply by -1 to make percentage positive for printing.
			fmt.Printf("For %s, HoldAvgPrice (%f) is greater than FairPrice (%f) by: %f (%.2f%%)\n", symbol, holdAvgPrice, fairPriceResp.Data.FairPrice, -difference, -percentageDifference)
		}
	}
}

func main() {
	accessKey := os.Getenv("MEXC_ACCESS_KEY")
	secretKey := os.Getenv("MEXC_SECRET_KEY")

	params := map[string]string{}
	paramStr := getRequestParamString(params)
	reqTime := strconv.FormatInt(time.Now().Unix()*1000, 10)

	signature := sign(accessKey, secretKey, reqTime, paramStr)

	baseURL := "https://contract.mexc.com"
	endpoint := "/api/v1/private/position/open_positions"

	fullURL := fmt.Sprintf("%s%s", baseURL, endpoint)

	client := &http.Client{}
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	req.Header.Add("ApiKey", accessKey)
	req.Header.Add("Request-Time", reqTime)
	req.Header.Add("Signature", signature)
	req.Header.Add("Content-Type", "application/json")

	response, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	var resp OpenPositionsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		fmt.Println("Error decoding response JSON:", err)
		return
	}

	for _, pos := range resp.Data {
		queryFairPriceForSymbol(client, accessKey, secretKey, baseURL, pos.Symbol, pos.HoldAvgPrice) // pos.HoldAvgPrice is now a float64
	}
}
