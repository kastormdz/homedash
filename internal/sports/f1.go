package sports

import (
	"encoding/json"
	"fmt"
	"homedash/internal/common"
	"homedash/internal/network"
	"time"
)

func GetCircuitData(circuitName string) string {
	return "/static/assets/circuits/" + circuitName + ".svg"
}
func GetFlagURL(iso string) string { return "/static/assets/flags/" + iso + ".svg" }

func fetchLiveF1() F1Race {
	resp, err := network.FetchSecure("https://api.jolpi.ca/ergast/f1/current/next.json")
	if err != nil {
		return F1Race{GrandPrix: "Sin carreras"}
	}
	defer resp.Body.Close()
	var data ergastResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil || len(data.MRData.RaceTable.Races) == 0 {
		return F1Race{GrandPrix: "Sin carreras"}
	}
	race := data.MRData.RaceTable.Races[0]

	// --- FETCH WEATHER FOR CIRCUIT ---
	weatherMap := make(map[string]string)
	if race.Circuit.Location.Lat != "" && race.Circuit.Location.Long != "" {
		wUrl := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%s&longitude=%s&daily=weather_code&timezone=auto&past_days=2", race.Circuit.Location.Lat, race.Circuit.Location.Long)
		if wResp, wErr := network.FetchSecure(wUrl); wErr == nil {
			defer wResp.Body.Close()
			var wData struct {
				Daily struct {
					Time        []string `json:"time"`
					WeatherCode []int    `json:"weather_code"`
				} `json:"daily"`
			}
			if json.NewDecoder(wResp.Body).Decode(&wData) == nil {
				for i, t := range wData.Daily.Time {
					code := wData.Daily.WeatherCode[i]
					icon := "sun"
					if code > 0 && code <= 3 {
						icon = "cloud"
					}
					if code >= 45 && code <= 48 {
						icon = "cloud-fog"
					}
					if code >= 51 && code <= 67 {
						icon = "cloud-rain"
					}
					if code >= 71 && code <= 77 {
						icon = "snowflake"
					}
					if code >= 80 && code <= 82 {
						icon = "cloud-rain"
					}
					if code >= 95 {
						icon = "cloud-lightning"
					}
					weatherMap[t] = icon
				}
			}
		}
	}
	// ---------------------------------

	sessions := []F1Session{}
	addSess := func(name, d, t string) {
		if d == "" || t == "" {
			return
		}
		s := F1Session{Name: name}
		if p, err := parseToArgentina(d + "T" + t); err == nil {
			s.Date = common.DaysAbbr[p.Weekday()] + " " + p.Format("02/01")
			s.Time = p.Format("15:04")
			s.Passed = p.Add(2 * time.Hour).Before(time.Now())
		} else {
			s.Date = d
			s.Time = t
		}
		s.WeatherIcon = weatherMap[d]
		sessions = append(sessions, s)
	}

	addSess("FP1", race.FirstPractice.Date, race.FirstPractice.Time)
	addSess("FP2", race.SecondPractice.Date, race.SecondPractice.Time)
	addSess("FP3", race.ThirdPractice.Date, race.ThirdPractice.Time)
	addSess("Qualy", race.Qualifying.Date, race.Qualifying.Time)
	addSess("Sprint", race.Sprint.Date, race.Sprint.Time)
	addSess("Race", race.Date, race.Time)

	return F1Race{GrandPrix: race.RaceName, Circuit: race.Circuit.CircuitId, FlagURL: GetFlagURL("un"), PosterURL: GetCircuitData(race.Circuit.CircuitId), Sessions: sessions}
}
