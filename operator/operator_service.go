package operator

import (
	cstaskmanager "avs-oracle/contracts/bindings/OpenOracleTaskManager"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
const DATA_SOURCE_COUNT = 3

const (
	Nasdaq = iota
	Cnbc
	Tradingview
	Insider
)

var sourceToWeight = map[uint8]float64{
	Nasdaq:      1.0,
	Cnbc:        2.0,
	Tradingview: 2.0,
	Insider:     0,
}

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

type TradingviewApiResponse struct {
	Close float64 `json:"close"`
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

var taskTypeToTradingviewId = map[uint8]string{
	Gold:       "COMEX:GC1!",
	Silver:     "COMEX:SI1!",
	Platinum:   "NYMEX:PL1!",
	Palladium:  "NYMEX:PA1!",
	Oil:        "NYMEX:CL1!",
	Copper:     "COMEX:HG1!",
	RbobGas:    "NYMEX:RB1!",
	NaturalGas: "NYMEX:NG1!",
	Brent:      "NYMEX:BB1!",
	Corn:       "CBOT:ZC1!",
	Soybean:    "CBOT:ZS1!",
	RoughRice:  "CBOT:ZR1!",
	Cocca:      "ICEUS:CC1!",
	Lumber:     "CME:LBR1!",
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

func (o *Operator) FetchPrice(taskType uint8) (int64, error) {
	// Generate a random number to randomly choose a data source
	var total float64 = 0.0
	var totalWeight float64 = 0
	var source uint8 = 0

	for ; source < DATA_SOURCE_COUNT; source++ {
		res, err := o.FetchPriceFromSource(taskType, source)
		if err != nil {
			o.logger.Error("Error fetching from source", "err", err)
			continue
		}
		total += res * sourceToWeight[source]
		totalWeight += sourceToWeight[source]
	}

	if totalWeight < 1.0 {
		return 0, errors.New("No price source fetched successfully")
	}

	return int64(total / totalWeight * 100), nil
}

func (o *Operator) FetchPriceFromSource(taskType uint8, source uint8) (float64, error) {
	switch source {
	case Cnbc:
		o.metrics.IncNumRequestToCnbc()
		cnbc_url := fmt.Sprintf(
			"https://quote.cnbc.com/quote-html-webservice/restQuote/symbolType/symbol?symbols=%%40%s",
			taskTypeToCnbcId[taskType],
		)
		// fmt.Println(cnbc_url)
		resp, err := http.Get(cnbc_url)
		if err != nil {
			o.metrics.IncNumErrorFromCnbc()
			return 0, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			o.metrics.IncNumErrorFromCnbc()
			return 0, err
		}
		var apiResponse CnbcApiResponse
		if err := json.Unmarshal(body, &apiResponse); err != nil {
			o.metrics.IncNumErrorFromCnbc()
			return 0, err
		}

		if len(apiResponse.FormattedQuoteResult.FormattedQuote) > 0 {
			val, err := strconv.ParseFloat(strings.Replace(apiResponse.FormattedQuoteResult.FormattedQuote[0].Last, ",", "", -1), 64)
			if err != nil {
				o.metrics.IncNumErrorFromCnbc()
				return 0, err
			}
			return val, nil
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
			return apiResponse[0].Close, nil
		}
	case Nasdaq:
		o.metrics.IncNumRequestToNasdaq()
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
			o.metrics.IncNumErrorFromNasdaq()
			return 0, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			o.metrics.IncNumErrorFromNasdaq()
			return 0, err
		}
		var apiResponse NasdaqApiResponse
		if err := json.Unmarshal(body, &apiResponse); err != nil {
			o.metrics.IncNumErrorFromNasdaq()
			return 0, err
		}

		priceStr := strings.Replace(apiResponse.Data.SummaryData.LastSalePrice.Value, ",", "", -1)
		priceStr = strings.Replace(priceStr, "$", "", -1)
		val, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			o.metrics.IncNumErrorFromNasdaq()
			return 0, err
		}
		return val, nil
	case Tradingview:
		o.metrics.IncNumRequestToTradingview()
		tradingview_url := fmt.Sprintf(
			"https://scanner.tradingview.com/symbol?fields=close&symbol=%s",
			taskTypeToTradingviewId[taskType],
		)
		// fmt.Println(tradingview_url)

		client := &http.Client{}
		req, _ := http.NewRequest("GET", tradingview_url, nil)
		// req.Header.Add("Accept", "*/*")

		resp, err := client.Do(req)
		if err != nil {
			o.metrics.IncNumErrorFromTradingview()
			return 0, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			o.metrics.IncNumErrorFromTradingview()
			return 0, err
		}
		var apiResponse TradingviewApiResponse
		if err := json.Unmarshal(body, &apiResponse); err != nil {
			o.metrics.IncNumErrorFromTradingview()
			return 0, err
		}
		return apiResponse.Close, nil
	default:
		return 0, errors.New("wrong pricefeed source")
	}
	return 0, errors.New("wrong pricefeed source")
}
