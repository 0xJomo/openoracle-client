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
	Copper
	RbobGas
	NaturalGas
	Brent
	Corn
	Soybean
	RoughRice
	Cocca
	Lumber
)

// Update here if add more data source
// Not using BusinessInsider source for now since it updates slowly
const DATA_SOURCE_COUNT = 2

const (
	Nasdaq = iota
	Cnbc
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

type NasdaqApiResponse struct {
	Data struct {
		SummaryData struct {
			LastSalePrice struct {
				Value string `json:"value"`
			} `json:"LastSalePrice"`
		} `json:"summaryData"`
	} `json:"data"`
}

var taskTypeToInsiderType = map[uint8]string{
	Gold:      "1",
	Silver:    "2",
	Platinum:  "3",
	Palladium: "4",
	Oil:       "6",
}

var taskTypeToCnbcId = map[uint8]string{
	Gold:       "GC.1",
	Silver:     "SI.1",
	Platinum:   "PL.1",
	Palladium:  "PA.1",
	Oil:        "CL.1",
	Copper:     "HG.1",
	RbobGas:    "RB.1",
	NaturalGas: "NG.1",
	Brent:      "LCO.1",
	Corn:       "C.1",
	Soybean:    "S.1",
	RoughRice:  "RR.1",
	Cocca:      "CC.1",
	Lumber:     "LBR.1",
}

var taskTypeToNasdaqId = map[uint8]string{
	Gold:       "GC%3ACMX",
	Silver:     "SI%3ACMX",
	Platinum:   "PL%3ANMX",
	Palladium:  "PA%3ANMX",
	Oil:        "CL%3ANMX",
	Copper:     "HG%3ACMX",
	RbobGas:    "RB%3ANMX",
	NaturalGas: "NG%3ANMX",
	Brent:      "BZ%3ANMX",
	Corn:       "ZC",
	Soybean:    "ZS",
	RoughRice:  "ZR",
	Cocca:      "CJ%3ANMX",
	Lumber:     "LBR",
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
	case Nasdaq:
		nasdaq_url := fmt.Sprintf(
			"https://api.nasdaq.com/api/quote/%s/summary?assetclass=commodities",
			taskTypeToNasdaqId[taskType],
		)
		// fmt.Println(nasdaq_url)

		client := &http.Client{}
		req, _ := http.NewRequest("GET", nasdaq_url, nil)
		req.Header.Add("Accept", "*/*")
		req.Header.Add("Accept-Language", "en-US,en;q=0.9")
		req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return 0, err
		}
		var apiResponse NasdaqApiResponse
		if err := json.Unmarshal(body, &apiResponse); err != nil {
			return 0, err
		}

		priceStr := strings.Replace(apiResponse.Data.SummaryData.LastSalePrice.Value, ",", "", -1)
		priceStr = strings.Replace(priceStr, "$", "", -1)
		val, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			return 0, err
		}
		return int64(val * 100), nil
	default:
		return 0, err
	}
	return 0, err
}
