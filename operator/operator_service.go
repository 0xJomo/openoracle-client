package operator

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type ApiResponse struct {
	SpreadProfilePrices []struct {
		Ask float64 `json:"ask"`
	} `json:"spreadProfilePrices"`
}

type TaskResponse struct {
	ChainName string `json:"chainName"`
	TaskId    string `json:"taskId"`
	Result    string `json:"result"`
	Timestamp int64  `json:"timestamp"`
}

type SignedTaskResponse struct {
	OperatorId   string       `json:"operatorId"`
	TaskResponse TaskResponse `json:"taskResponse"`
	PublicKey    string       `json:"publicKey"`
	Signature    string       `json:"signature"`
}

func SendBlsSignedRequest(payload SignedTaskResponse, url string) error {

	// Serialize the payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Send the request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
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
		return int64(apiResponse[0].SpreadProfilePrices[0].Ask * 100), nil
	}

	return 0, nil // No data available
}
