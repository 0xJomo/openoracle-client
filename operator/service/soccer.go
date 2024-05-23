package service

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"net/http"
	"net/url"
	"strconv"
)

const (
	rapidAPIKey  = "9dc27740a8msh7c20f698e7c01dep1bb9f5jsna5734f460b94"
)

type TeamStatisticsResponse struct {
	Get        string `json:"get"`
	Parameters struct {
		League string `json:"league"`
		Season string `json:"season"`
		Team   string `json:"team"`
	} `json:"parameters"`
	Errors   []interface{} `json:"errors"`
	Results  int           `json:"results"`
	Paging   struct {
		Current int `json:"current"`
		Total   int `json:"total"`
	} `json:"paging"`
	Response struct {
		League struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			Country string `json:"country"`
			Logo    string `json:"logo"`
			Flag    string `json:"flag"`
			Season  int    `json:"season"`
		} `json:"league"`
		Team struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Logo string `json:"logo"`
		} `json:"team"`
		Form          string `json:"form"`
		Fixtures      Fixtures `json:"fixtures"`
		Goals         Goals `json:"goals"`
		Biggest       Biggest `json:"biggest"`
		CleanSheet    CleanSheet `json:"clean_sheet"`
		FailedToScore FailedToScore `json:"failed_to_score"`
		Penalty       Penalty `json:"penalty"`
		Lineups       []Lineup `json:"lineups"`
		Cards         Cards `json:"cards"`
	} `json:"response"`
}

type Fixtures struct {
	Played Played `json:"played"`
	Wins   Wins `json:"wins"`
}

type Played struct {
	Home  int `json:"home"`
	Away  int `json:"away"`
	Total int `json:"total"`
}

type Wins struct {
	Home  int `json:"home"`
	Away  int `json:"away"`
	Total int `json:"total"`
}

type Goals struct {
	For struct {
		Total   Total `json:"total"`
		Average Average `json:"average"`
		Minute  Minute `json:"minute"`
	} `json:"for"`
	Against struct {
		Total   Total `json:"total"`
		Average Average `json:"average"`
		Minute  Minute `json:"minute"`
	} `json:"against"`
}

type Total struct {
	Home  int `json:"home"`
	Away  int `json:"away"`
	Total int `json:"total"`
}

type Average struct {
	Home  string `json:"home"`
	Away  string `json:"away"`
	Total string `json:"total"`
}

type Minute struct {
	_0_15   SubMinute `json:"0-15"`
	_16_30  SubMinute `json:"16-30"`
	_31_45  SubMinute `json:"31-45"`
	_46_60  SubMinute `json:"46-60"`
	_61_75  SubMinute `json:"61-75"`
	_76_90  SubMinute `json:"76-90"`
	_91_105 SubMinute `json:"91-105"`
	_106_120 SubMinute `json:"106-120"`
}

type SubMinute struct {
	Total      int    `json:"total"`
	Percentage string `json:"percentage"`
}

type Biggest struct {
	Streak Streak `json:"streak"`
	Wins   struct {
		Home string `json:"home"`
		Away string `json:"away"`
	} `json:"wins"`
	Loses struct {
		Home string `json:"home"`
		Away string `json:"away"`
	} `json:"loses"`
	Goals struct {
		For     Total `json:"for"`
		Against Total `json:"against"`
	} `json:"goals"`
}

type Streak struct {
	Wins  int `json:"wins"`
	Draws int `json:"draws"`
	Loses int `json:"loses"`
}

type CleanSheet struct {
	Home  int `json:"home"`
	Away  int `json:"away"`
	Total int `json:"total"`
}

type FailedToScore struct {
	Home  int `json:"home"`
	Away  int `json:"away"`
	Total int `json:"total"`
}

type Penalty struct {
	Scored  Total   `json:"scored"`
	Missed  Total   `json:"missed"`
	Total   int     `json:"total"`
}

type Lineup struct {
	Formation string `json:"formation"`
	Played    int    `json:"played"`
}

type Cards struct {
	Yellow map[string]SubMinute `json:"yellow"`
	Red    map[string]SubMinute `json:"red"`
}

// FetchTeamStatistics fetches team statistics for a given season and team ID
func FetchTeamStatisticsFromSportsWithUrl(urlStr string) (int, int, int, int, error) {
	//fmt.Println("Fetching from url:", urlStr)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("error sending request: %v", err)
	}
	urlStrURL, err := url.Parse(urlStr)
	req.Header.Add("X-RapidAPI-Key", rapidAPIKey)
	req.Header.Add("X-RapidAPI-Host", urlStrURL.Host)

	client := &http.Client{}

	res, err := client.Do(req)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("unexpected response status code: %d", res.StatusCode)
	}
	defer res.Body.Close()

	var teamStats TeamStatisticsResponse
	err = json.NewDecoder(res.Body).Decode(&teamStats)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("error decoding response: %v", err)
	}

	GS := teamStats.Response.Goals.For.Total.Total
	MW := teamStats.Response.Fixtures.Wins.Total
	CS := teamStats.Response.CleanSheet.Total - teamStats.Response.FailedToScore.Total
	GD := teamStats.Response.Goals.For.Total.Total - teamStats.Response.Goals.Against.Total.Total
	return GS, MW, CS, GD, nil
}

func FetchTeamValueWithUrl(linkConfig map[string]string, taskData []byte) (int, error) {
	//0000002700000000000007e700000021
	hexData := common.Bytes2Hex(taskData)
	//Assuming league ID is always 39
	league, err := strconv.ParseUint(hexData[:8], 16, 32)
	season, err:= strconv.ParseUint(hexData[8:24], 16, 64)
	teamId, err:= strconv.ParseUint(hexData[24:], 16, 32)

	teamValue  := 0
	urlStr := fmt.Sprintf(linkConfig["rapid"], league, season, teamId)
	GS_sports, MW_sports, CS_sports, GD_sports, err := FetchTeamStatisticsFromSportsWithUrl(urlStr)
	if err != nil {
		fmt.Println(err)
		return 0, err // Return immediately if there's an error
	}

	//fmt.Println("****Season****:", season)
	//fmt.Println("Goals Scored:", GS_sports)
	//fmt.Println("Matches Won:", MW_sports)
	//fmt.Println("Clean Sheets:", CS_sports)
	//fmt.Println("Goal Difference:", GD_sports)

	teamValue += GS_sports + MW_sports + CS_sports + GD_sports


	return teamValue, nil
}