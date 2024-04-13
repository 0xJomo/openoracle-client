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
	"strconv"
	"strings"
	"time"
)

const (
	Oil = iota // Oil = 0
	Gold
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
			Last string `json:"last"`
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
	bigNum, err := rand.Int(rand.Reader, big.NewInt(DATA_SOURCE_COUNT))
	if err != nil {
		return 0, err
	}
	switch bigNum.Int64() {
	case Cnbc:
		cnbc_url := fmt.Sprintf(
			"https://quote.cnbc.com/quote-html-webservice/restQuote/symbolType/symbol?symbols=%%40%s",
			taskTypeToCnbcId[taskType],
		)
		// fmt.Println(cnbc_url)
		resp, err := http.Get(cnbc_url)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return 0, err
		}
		var apiResponse CnbcApiResponse
		if err := json.Unmarshal(body, &apiResponse); err != nil {
			return 0, err
		}

		if len(apiResponse.FormattedQuoteResult.FormattedQuote) > 0 {
			val, err := strconv.ParseFloat(strings.Replace(apiResponse.FormattedQuoteResult.FormattedQuote[0].Last, ",", "", -1), 64)
			if err != nil {
				return 0, err
			}
			return int64(val * 100), nil
		}
	case Insider:
		now := time.Now()
		yesterday := now.AddDate(0, 0, -1)
		// Format the date into YYYYMMDD type
		formatted := fmt.Sprintf("%d%02d%02d", now.Year(), now.Month(), now.Day())
		formatted_yesterday := fmt.Sprintf("%d%02d%02d", yesterday.Year(), yesterday.Month(), yesterday.Day())
		insider_url := fmt.Sprintf(
			"https://markets.businessinsider.com/Ajax/Chart_GetChartData?instrumentType=Commodity&tkData=300002,%s,0,333&from=%s&to=%s",
			taskTypeToInsiderType[taskType],
			formatted_yesterday,
			formatted,
		)
		// fmt.Println(insider_url)
		resp, err := http.Get(insider_url)
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
