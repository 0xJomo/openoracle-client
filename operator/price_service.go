package operator

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type ApiResponse struct {
	SpreadProfilePrices []struct {
		Ask float64 `json:"ask"`
	} `json:"spreadProfilePrices"`
}

// FetchGoldPrice function fetches the first gold ask price from the API
func FetchGoldPrice() (int64, error) {
	resp, err := http.Get("https://forex-data-feed.swissquote.com/public-quotes/bboquotes/instrument/XAU/USD")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var apiResponse []ApiResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return 0, err
	}

	if len(apiResponse) > 0 && len(apiResponse[0].SpreadProfilePrices) > 0 {
		return int64(apiResponse[0].SpreadProfilePrices[0].Ask), nil
	}

	return 0, nil // No data available
}
