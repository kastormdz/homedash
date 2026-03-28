package sports

import (
	"context"
	"fmt"
	"homedash/internal/common"
	"log"
	"strings"
	"sync"
	"time"
)

var (
	cachedData SportsData
	cacheMutex sync.RWMutex
)

func init() {
	cachedData = SportsData{
		AllMatches: []MatchData{},
		F1:         F1Race{GrandPrix: "Cargando..."},
		UFC:        UFCMatch{EventName: "Cargando..."},
	}
}

func StartUpdateLoop() {
	go func() {
		for {
			_, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			ForceUpdate()
			cancel()

			// Determinar próximo intervalo
			interval := 15 * time.Minute

			cacheMutex.RLock()
			hasLive := false
			for _, m := range cachedData.AllMatches {
				if m.Status == "LIVE" {
					hasLive = true
					break
				}
			}
			cacheMutex.RUnlock()

			if hasLive {
				interval = 1 * time.Minute
				log.Printf("[SPORTS] Hay partidos en vivo. Próxima actualización en 1 min.")
			}

			time.Sleep(interval)
		}
	}()
}

func ForceUpdate() error {
	start := time.Now()
	newData := fetchFreshSportsData()

	// Si no obtuvimos nada de nada (ni F1, ni UFC, ni partidos), podrías ser un error de red
	if len(newData.AllMatches) == 0 && (newData.F1.GrandPrix == "Sin carreras" || newData.F1.GrandPrix == "Cargando...") && (newData.UFC.EventName == "Sin eventos" || newData.UFC.EventName == "Cargando...") {
		return fmt.Errorf("no se pudo obtener ningún dato de deportes (posible error de red)")
	}

	cacheMutex.Lock()
	cachedData = newData
	cacheMutex.Unlock()
	log.Printf("[SPORTS] Update completado en %v. Partidos: %d\n", time.Since(start), len(newData.AllMatches))
	return nil
}

func GetSportsData() SportsData {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	return cachedData
}

func fetchFreshSportsData() SportsData {
	var wg sync.WaitGroup
	wg.Add(3)

	var f1Data F1Race
	var ufcData UFCMatch
	var promiedosList []PromiedosMatch

	go func() {
		defer wg.Done()
		f1Data = fetchLiveF1()
	}()

	go func() {
		defer wg.Done()
		ufcData = fetchLiveUFC()
	}()

	go func() {
		defer wg.Done()
		promiedosList = fetchPromiedosChannels()
	}()

	wg.Wait()

	promiedosLookup := make(map[string]PromiedosMatch)
	for _, pm := range promiedosList {
		h := common.NormalizeName(pm.Home)
		a := common.NormalizeName(pm.Away)
		promiedosLookup[h+"|"+a] = pm
		promiedosLookup[a+"|"+h] = pm
	}

	var allMatches []MatchData
	now := time.Now()
	todayStr := now.Format("02/01")
	later := now.AddDate(0, 0, 15)
	dateRange := now.Format("20060102") + "-" + later.Format("20060102")

	urls := []string{
		"https://site.api.espn.com/apis/site/v2/sports/soccer/arg.1/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
		"https://site.api.espn.com/apis/site/v2/sports/soccer/arg.2/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
		"https://site.api.espn.com/apis/site/v2/sports/soccer/arg.copa/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
		"https://site.api.espn.com/apis/site/v2/sports/soccer/lib/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
		"https://site.api.espn.com/apis/site/v2/sports/soccer/sud.copa/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
	}

	var mu sync.Mutex
	wg.Add(len(urls))
	for _, u := range urls {
		go func(url string) {
			defer wg.Done()
			if m := fetchLiveMatches(url); m != nil {
				for i := range m {
					if strings.Contains(m[i].Date, todayStr) {
						pTeam := common.NormalizeName(m[i].Team)
						pOpp := common.NormalizeName(m[i].Opponent)

						if val, ok := promiedosLookup[pTeam+"|"+pOpp]; ok {
							if val.Channel != "" {
								m[i].Channel = val.Channel
							}
							if val.HScore != "" {
								m[i].HomeScore = val.HScore
							}
							if val.AScore != "" {
								m[i].AwayScore = val.AScore
							}
							if val.Status != "" {
								m[i].Status = val.Status
							}
							if val.Clock != "" && val.Clock != "0'" {
								m[i].Clock = val.Clock
							}

							m[i].Channel = strings.ReplaceAll(m[i].Channel, "Premium", "")
							m[i].Channel = strings.TrimSpace(m[i].Channel)
						}
					}
				}
				mu.Lock()
				allMatches = append(allMatches, m...)
				mu.Unlock()
			}
		}(u)
	}
	wg.Wait()

	return SportsData{AllMatches: allMatches, F1: f1Data, UFC: ufcData}
}
