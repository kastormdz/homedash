package sports

import (
	"context"
	"encoding/json"
	"fmt"
	"homedash/internal/common"
	"homedash/internal/network"
	"strings"
	"sync"
	"time"
)

func GetCircuitData(circuitName string) string {
	mapping := map[string]string{
		"villeneuve":    "gilles_villeneuve",
		"catalunya":     "barcelona_catalunya",
		"americas":      "cota",
		"bahrain":       "sakhir",
		"rodriguez":     "hermanos_rodriguez",
		"spa":           "spa_francorchamps",
		"losail":        "lusail",
	}
	if mapped, ok := mapping[circuitName]; ok {
		circuitName = mapped
	}
	return "/static/assets/circuits/" + circuitName + ".svg"
}

func GetFlagURL(country string) string {
	country = strings.ToLower(strings.TrimSpace(country))
	iso := "un" // Unknown
	mapping := map[string]string{
		"australia": "au", "austria": "at", "azerbaijan": "az", "belgium": "be",
		"brazil": "br", "canada": "ca", "china": "cn", "hungary": "hu",
		"italy": "it", "japan": "jp", "monaco": "mc", "mexico": "mx",
		"netherlands": "nl", "qatar": "qa", "saudi arabia": "sa", "singapore": "sg",
		"spain": "es", "uae": "ae", "uk": "gb", "usa": "us", "bahrain": "bh",
		"united kingdom": "gb", "united states": "us",
	}
	if code, ok := mapping[country]; ok {
		iso = code
	}
	return "/static/assets/flags/" + iso + ".svg"
}

var (
	f1WeatherCache     map[string]string
	f1WeatherCacheTime time.Time
	f1WeatherMutex     sync.RWMutex
)

func getF1Weather(ctx context.Context, lat, long string) map[string]string {
	f1WeatherMutex.RLock()
	if f1WeatherCache != nil && time.Since(f1WeatherCacheTime) < 1*time.Hour {
		// Asumimos que el clima cambia poco para la misma ubicación
		res := f1WeatherCache
		f1WeatherMutex.RUnlock()
		return res
	}
	f1WeatherMutex.RUnlock()

	weatherMap := make(map[string]string)
	wUrl := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%s&longitude=%s&daily=weather_code&timezone=auto", lat, long)
	
	wResp, wErr := network.FetchSecureWithContext(ctx, wUrl)
	if wErr != nil {
		if wResp != nil && wResp.Body != nil {
			wResp.Body.Close()
		}
		return weatherMap
	}
	defer wResp.Body.Close()

	var wData struct {
		Daily struct {
			Time        []string `json:"time"`
			WeatherCode []int    `json:"weather_code"`
		} `json:"daily"`
	}
	if json.NewDecoder(wResp.Body).Decode(&wData) == nil {
		for i, t := range wData.Daily.Time {
			if i >= len(wData.Daily.WeatherCode) {
				break
			}
			code := wData.Daily.WeatherCode[i]
			icon := "sun"
			if code > 0 && code <= 3 {
				icon = "cloud"
			} else if code >= 45 && code <= 48 {
				icon = "cloud-fog"
			} else if code >= 51 && code <= 67 {
				icon = "cloud-rain"
			} else if code >= 71 && code <= 77 {
				icon = "snowflake"
			} else if code >= 80 && code <= 82 {
				icon = "cloud-rain"
			} else if code >= 95 {
				icon = "cloud-lightning"
			}
			weatherMap[t] = icon
		}
	}

	f1WeatherMutex.Lock()
	f1WeatherCache = weatherMap
	f1WeatherCacheTime = time.Now()
	f1WeatherMutex.Unlock()

	return weatherMap
}

func fetchLiveF1(ctx context.Context) F1Race {
	resp, err := network.FetchSecureWithContext(ctx, "https://api.jolpi.ca/ergast/f1/current/next.json")
	if err != nil {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return F1Race{GrandPrix: "Sin carreras"}
	}
	defer resp.Body.Close()
	var data ergastResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil || len(data.MRData.RaceTable.Races) == 0 {
		return F1Race{GrandPrix: "Sin carreras"}
	}
	race := data.MRData.RaceTable.Races[0]

	weatherMap := make(map[string]string)
	if race.Circuit.Location.Lat != "" && race.Circuit.Location.Long != "" {
		weatherMap = getF1Weather(ctx, race.Circuit.Location.Lat, race.Circuit.Location.Long)
	}

	sessions := []F1Session{}
	addSess := func(name, d, t string) {
		if d == "" || t == "" {
			return
		}
		s := F1Session{Name: name}
		// S-61. Usar la fecha original 'd' para el clima
		icon, ok := weatherMap[d]
		if !ok {
			icon = "sun" // Default si la API no tiene ese día (común en carreras lejanas)
		}
		s.WeatherIcon = icon

		if p, err := parseToArgentina(d + "T" + t); err == nil {
			s.Date = common.DaysAbbr[p.Weekday()] + " " + p.Format("02/01")
			s.Time = p.Format("15:04")
			s.Passed = p.Add(2 * time.Hour).Before(time.Now())
		} else {
			s.Date = d
			s.Time = t
		}
		sessions = append(sessions, s)
	}

	addSess("FP1", race.FirstPractice.Date, race.FirstPractice.Time)
	addSess("FP2", race.SecondPractice.Date, race.SecondPractice.Time)
	addSess("FP3", race.ThirdPractice.Date, race.ThirdPractice.Time)
	addSess("SQualy", race.SprintQualifying.Date, race.SprintQualifying.Time)
	addSess("Sprint", race.Sprint.Date, race.Sprint.Time)
	addSess("Qualy", race.Qualifying.Date, race.Qualifying.Time)
	addSess("Race", race.Date, race.Time)

	return F1Race{
		GrandPrix: race.RaceName,
		Circuit:   race.Circuit.CircuitId,
		FlagURL:   GetFlagURL(race.Circuit.Location.Country),
		PosterURL: GetCircuitData(race.Circuit.CircuitId),
		Sessions:  sessions,
	}
}

func FetchF1Standings(ctx context.Context) (F1Standings, error) {
	var standings F1Standings
	var wg sync.WaitGroup
	wg.Add(2)

	var errD, errC error

	// 1. Pilotos
	go func() {
		defer wg.Done()
		resp, err := network.FetchSecureWithContext(ctx, "https://api.jolpi.ca/ergast/f1/current/driverStandings.json")
		if err != nil {
			errD = err
			return
		}
		defer resp.Body.Close()

		var data struct {
			MRData struct {
				StandingsTable struct {
					StandingsLists []struct {
						DriverStandings []struct {
							Position string `json:"position"`
							Points   string `json:"points"`
							Wins     string `json:"wins"`
							Driver   struct {
								FamilyName string `json:"familyName"`
							} `json:"Driver"`
							Constructors []struct {
								Name string `json:"name"`
							} `json:"Constructors"`
						} `json:"DriverStandings"`
					} `json:"StandingsLists"`
				} `json:"StandingsTable"`
			} `json:"MRData"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&data); err == nil && len(data.MRData.StandingsTable.StandingsLists) > 0 {
			for _, d := range data.MRData.StandingsTable.StandingsLists[0].DriverStandings {
				team := "N/A"
				if len(d.Constructors) > 0 {
					team = d.Constructors[0].Name
				}
				standings.Drivers = append(standings.Drivers, F1DriverStanding{
					Pos:    d.Position,
					Driver: d.Driver.FamilyName,
					Team:   team,
					Points: d.Points,
					Wins:   d.Wins,
				})
			}
		}
	}()

	// 2. Constructores
	go func() {
		defer wg.Done()
		resp, err := network.FetchSecureWithContext(ctx, "https://api.jolpi.ca/ergast/f1/current/constructorStandings.json")
		if err != nil {
			errC = err
			return
		}
		defer resp.Body.Close()

		var data struct {
			MRData struct {
				StandingsTable struct {
					StandingsLists []struct {
						ConstructorStandings []struct {
							Position    string `json:"position"`
							Points      string `json:"points"`
							Wins        string `json:"wins"`
							Constructor struct {
								Name string `json:"name"`
							} `json:"Constructor"`
						} `json:"ConstructorStandings"`
					} `json:"StandingsLists"`
				} `json:"StandingsTable"`
			} `json:"MRData"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&data); err == nil && len(data.MRData.StandingsTable.StandingsLists) > 0 {
			for _, c := range data.MRData.StandingsTable.StandingsLists[0].ConstructorStandings {
				standings.Constructors = append(standings.Constructors, F1ConstructorStanding{
					Pos:    c.Position,
					Team:   c.Constructor.Name,
					Points: c.Points,
					Wins:   c.Wins,
				})
			}
		}
	}()

	wg.Wait()

	if errD != nil && errC != nil {
		return standings, fmt.Errorf("error fetching standings: %v, %v", errD, errC)
	}

	return standings, nil
}
