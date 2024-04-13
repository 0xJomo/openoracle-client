package operator

import (
	cstaskmanager "avs-oracle/contracts/bindings/OpenOracleTaskManager"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"
)

const (
	Gold = iota // Gold = 0
	Oil
	Silver
	Platinum
	Palladium
)

// Update here if add more data source
const DATA_SOURCE_COUNT = 2

const (
	Cnbc = iota
	Insider
)

type CnbcApiResponse struct {
	FormattedQuoteResult struct {
		FormattedQuote []struct {
			Last float64 `json:"last"`
		} `json:"FormattedQuote"`
	} `json:"FormattedQuoteResult"`
}

type InsiderApiResponse struct {
	Close float64 `json:"Close"`
	Open  float64 `json:"Open"`
}

var taskTypeToInsiderType = map[uint8]string{
	Gold:      "1",
	Silver:    "2",
	Platinum:  "3",
	Palladium: "4",
	Oil:       "6",
}

var taskTypeToCnbcId = map[uint8]string{
	Gold:      "GC.1",
	Silver:    "SI.1",
	Platinum:  "PL.1",
	Palladium: "PA.1",
	Oil:       "CL.1",
}

type ApiResponse struct {
	SpreadProfilePrices []struct {
		Ask float64 `json:"ask"`
	} `json:"spreadProfilePrices"`
}

type SignedTaskResponse struct {
	ChainName       string                                            `json:"chainName"`
	TaskResponse    *cstaskmanager.IOpenOracleTaskManagerTaskResponse `json:"taskResponse"`
	OperatorAddress string                                            `json:"operatorAddress"`
	Signature       string                                            `json:"signature"`
}

func SendECDSASignedRequest(payload SignedTaskResponse, url string) error {

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

	body, err := io.ReadAll(resp.Body)
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

func FetchPrice(taskType uint8) (int64, error) {
	// Generate a random number to randomly choose a data source
	bigNum, err := rand.Int(rand.Reader, big.NewInt(2))
	if err != nil {
		return 0, err
	}
	switch bigNum.Int64() {
	case Cnbc:
		resp, err := http.Get(fmt.Sprintf("https://quote.cnbc.com/quote-html-webservice/restQuote/symbolType/symbol?symbols=%40%s",
			taskTypeToCnbcId[taskType]))
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return 0, err
		}
		var apiResponse []CnbcApiResponse
		if err := json.Unmarshal(body, &apiResponse); err != nil {
			return 0, err
		}
		if len(apiResponse) > 0 && len(apiResponse[0].FormattedQuoteResult.FormattedQuote) > 0 {
			return int64(apiResponse[0].FormattedQuoteResult.FormattedQuote[0].Last * 100), nil
		}
	case Insider:
		now := time.Now()
		// Format the date into YYYYMMDD type
		formatted := fmt.Sprintf("%d%02d%02d", now.Year(), now.Month(), now.Day())
		resp, err := http.Get(
			fmt.Sprintf("https://markets.businessinsider.com/Ajax/Chart_GetChartData?instrumentType=Commodity&tkData=300002,%s,0,333&from=%s&to=%s",
				taskTypeToInsiderType[taskType],
				formatted,
				formatted))
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return 0, err
		}
		var apiResponse []InsiderApiResponse
		if err := json.Unmarshal(body, &apiResponse); err != nil {
			return 0, err
		}
		if len(apiResponse) > 0 {
			return int64(apiResponse[0].Close * 100), nil
		}
	default:
		return 0, err
	}
	return 0, err
}
