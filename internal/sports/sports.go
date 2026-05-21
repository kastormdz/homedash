package sports

import (
	"context"
	"encoding/json"
	"fmt"
	"homedash/internal/common"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

const cacheFilePath = "/app/data/sports_cache.json"

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
	loadSportsCache()
}

func loadSportsCache() {
	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[SPORTS] Error leyendo cache: %v", err)
		}
		return
	}
	var loaded SportsData
	if err := json.Unmarshal(data, &loaded); err != nil {
		log.Printf("[SPORTS] Error parseando cache: %v", err)
		return
	}
	cachedData = loaded
	log.Printf("[SPORTS] Cache cargado desde disco: %d partidos, GP=%s", len(loaded.AllMatches), loaded.F1.GrandPrix)
}

func saveSportsCache(data SportsData) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Printf("[SPORTS] Error serializando cache: %v", err)
		return
	}
	if err := os.WriteFile(cacheFilePath, b, 0644); err != nil {
		log.Printf("[SPORTS] Error escribiendo cache: %v", err)
	}
}

func StartUpdateLoop() {
	go func() {
		// Ya hicimos ForceUpdate en main(), así que primero dormimos
		for {
			// Determinar próximo intervalo (15 min por defecto, 1 min si hay actividad)
			interval := 15 * time.Minute

			cacheMutex.RLock()
			needsQuickUpdate := false
			for _, m := range cachedData.AllMatches {
				if m.Status == "LIVE" {
					needsQuickUpdate = true
					break
				}
				// Si falta menos de 30 min para que empiece un partido
				if !m.RawDate.IsZero() && time.Until(m.RawDate) > 0 && time.Until(m.RawDate) < 30*time.Minute {
					needsQuickUpdate = true
					log.Printf("[SPORTS] Partido por comenzar: %s vs %s en %v", m.Team, m.Opponent, time.Until(m.RawDate).Round(time.Minute))
					break
				}
			}
			cacheMutex.RUnlock()

			if needsQuickUpdate {
				interval = 1 * time.Minute
			}

			time.Sleep(interval)

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			ForceUpdate(ctx)
			cancel()
		}
	}()
}

func ForceUpdate(ctx context.Context) error {
	start := time.Now()
	newData := fetchFreshSportsData(ctx)

	// Si no obtuvimos nada de nada (ni F1, ni UFC, ni partidos), podrías ser un error de red
	if len(newData.AllMatches) == 0 && (newData.F1.GrandPrix == "Sin carreras" || newData.F1.GrandPrix == "Cargando...") && (newData.UFC.EventName == "Sin eventos" || newData.UFC.EventName == "Cargando...") {
		return fmt.Errorf("no se pudo obtener ningún dato de deportes (posible error de red)")
	}

	cacheMutex.Lock()
	oldData := cachedData
	detectReschedules(&newData, oldData)
	cachedData = newData
	cacheMutex.Unlock()

	saveSportsCache(newData)

	log.Printf("[SPORTS] Update completado en %v. Partidos: %d\n", time.Since(start), len(newData.AllMatches))
	return nil
}

func detectReschedules(newData *SportsData, oldData SportsData) {
	// Football: comparar por EventID
	oldFootball := make(map[string]MatchData)
	for _, m := range oldData.AllMatches {
		if m.EventID != "" {
			oldFootball[m.EventID] = m
		}
	}
	for i := range newData.AllMatches {
		newM := &newData.AllMatches[i]
		if oldM, ok := oldFootball[newM.EventID]; ok {
			if oldM.Date != newM.Date || oldM.Time != newM.Time {
				newM.WasRescheduled = true
				newM.RescheduledNote = fmt.Sprintf("Antes: %s %s", oldM.Date, oldM.Time)
				log.Printf("[RESCHEDULE] Fútbol: %s vs %s → %s %s (era %s %s)", newM.Team, newM.Opponent, newM.Date, newM.Time, oldM.Date, oldM.Time)
			}
		}
	}

	// F1: comparar sesiones por Gran Premio + Nombre de sesión
	oldF1 := make(map[string]F1Session)
	for _, s := range oldData.F1.Sessions {
		key := oldData.F1.GrandPrix + "|" + s.Name
		oldF1[key] = s
	}
	for i := range newData.F1.Sessions {
		newS := &newData.F1.Sessions[i]
		key := newData.F1.GrandPrix + "|" + newS.Name
		if oldS, ok := oldF1[key]; ok {
			if oldS.Date != newS.Date || oldS.Time != newS.Time {
				newS.WasRescheduled = true
				newS.RescheduledNote = fmt.Sprintf("Antes: %s %s", oldS.Date, oldS.Time)
				log.Printf("[RESCHEDULE] F1: %s - %s → %s %s (era %s %s)", newData.F1.GrandPrix, newS.Name, newS.Date, newS.Time, oldS.Date, oldS.Time)
			}
		}
	}

	// UFC: comparar por EventName
	if oldData.UFC.EventName != "" && oldData.UFC.EventName == newData.UFC.EventName {
		if oldData.UFC.Date != newData.UFC.Date || oldData.UFC.Time != newData.UFC.Time {
			newData.UFC.WasRescheduled = true
			newData.UFC.RescheduledNote = fmt.Sprintf("Antes: %s %s", oldData.UFC.Date, oldData.UFC.Time)
			log.Printf("[RESCHEDULE] UFC: %s → %s %s (era %s %s)", newData.UFC.EventName, newData.UFC.Date, newData.UFC.Time, oldData.UFC.Date, oldData.UFC.Time)
		}
	}

	// WorldCup: comparar por ID
	oldWC := make(map[string]WorldCupMatch)
	for _, m := range oldData.WorldCup {
		if m.ID != "" {
			oldWC[m.ID] = m
		}
	}
	for i := range newData.WorldCup {
		newM := &newData.WorldCup[i]
		if oldM, ok := oldWC[newM.ID]; ok {
			if oldM.Date != newM.Date || oldM.Time != newM.Time {
				newM.WasRescheduled = true
				newM.RescheduledNote = fmt.Sprintf("Antes: %s %s", oldM.Date, oldM.Time)
				log.Printf("[RESCHEDULE] Mundial: %s vs %s → %s %s (era %s %s)", newM.Home, newM.Away, newM.Date, newM.Time, oldM.Date, oldM.Time)
			}
		}
	}
}

func GetSportsData() SportsData {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	return cachedData
}

func fetchFreshSportsData(ctx context.Context) SportsData {
	var wg sync.WaitGroup
	var fetchMu sync.Mutex
	wg.Add(4)

	var f1Data F1Race
	var ufcData UFCMatch
	var wcData []WorldCupMatch
	var promiedosList []PromiedosMatch

	go func() {
		defer wg.Done()
		data := fetchLiveF1(ctx)
		fetchMu.Lock()
		f1Data = data
		fetchMu.Unlock()
	}()

	go func() {
		defer wg.Done()
		data := fetchLiveUFC(ctx)
		fetchMu.Lock()
		ufcData = data
		fetchMu.Unlock()
	}()

	go func() {
		defer wg.Done()
		data := fetchWorldCupFixture(ctx)
		fetchMu.Lock()
		wcData = data
		fetchMu.Unlock()
	}()

	go func() {
		defer wg.Done()
		data := fetchPromiedosChannels(ctx)
		fetchMu.Lock()
		promiedosList = data
		fetchMu.Unlock()
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
	later := now.AddDate(0, 0, 15)
	dateRange := now.Format("20060102") + "-" + later.Format("20060102")

	urls := []string{
		"https://site.api.espn.com/apis/site/v2/sports/soccer/arg.1/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
		"https://site.api.espn.com/apis/site/v2/sports/soccer/arg.2/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
		"https://site.api.espn.com/apis/site/v2/sports/soccer/arg.copa/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
		"https://site.api.espn.com/apis/site/v2/sports/soccer/arg.copa_lpf/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
		"https://site.api.espn.com/apis/site/v2/sports/soccer/conmebol.libertadores/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
		"https://site.api.espn.com/apis/site/v2/sports/soccer/conmebol.sudamericana/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
	}

	var mu sync.Mutex
	wg.Add(len(urls))
	for _, u := range urls {
		go func(url string) {
			defer wg.Done()
			if m := fetchLiveMatches(ctx, url); m != nil {
				for i := range m {
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
				mu.Lock()
				allMatches = append(allMatches, m...)
				mu.Unlock()
			}
		}(u)
	}
	wg.Wait()

	return SportsData{AllMatches: allMatches, F1: f1Data, UFC: ufcData, WorldCup: wcData}
}
