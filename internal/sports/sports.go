package sports

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	cachedData   SportsData
	cacheMutex   sync.RWMutex
)

func init() {
	cachedData = SportsData{
		AllMatches: []MatchData{},
		F1:         F1Race{GrandPrix: "Cargando..."},
		UFC:        UFCMatch{EventName: "Cargando..."},
	}
	
	go func() {
		for {
			ForceUpdate()
			
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

func ForceUpdate() {
	start := time.Now()
	newData := fetchFreshSportsData()
	cacheMutex.Lock()
	cachedData = newData
	cacheMutex.Unlock()
	fmt.Printf("[SPORTS] Update completado en %v. Partidos: %d\n", time.Since(start), len(newData.AllMatches))
}

type MatchData struct {
	Team          string
	Opponent      string
	Date          string
	Time          string
	Tournament    string
	Stadium       string
	TeamCrest     string
	OpponentCrest string
	Channel       string
	HomeScore     string
	AwayScore     string
	Status        string
	Clock         string
}

type F1Session struct {
	Name        string
	Date        string
	Time        string
	WeatherIcon string
}

type F1Race struct {
	GrandPrix string
	Circuit   string
	Country   string
	FlagURL   string
	PosterURL string
	Sessions  []F1Session
}

type UFCMatch struct {
	EventName string
	Date      string
	Time      string
	Status    string
	MainCard  []string
}

type SportsData struct {
	AllMatches []MatchData
	F1         F1Race
	UFC        UFCMatch
}

type UserSportsData struct {
	Match MatchData
	F1    F1Race
	UFC   UFCMatch
}

func GetSportsDataForUser(teamName string) UserSportsData {
	all := GetSportsData()
	searchName := normalizeName(teamName)
	if searchName == "" { searchName = "boca" }

	var bestMatch MatchData
	found := false
	lowestScore := 999 // Ahora usamos un score donde menor es mejor

	for _, f := range all.AllMatches {
		tName := normalizeName(f.Team)
		oName := normalizeName(f.Opponent)
		tourn := strings.ToLower(f.Tournament)

		matchTeam := false
		// Desambiguación para Independiente
		if searchName == "independiente" {
			if (tName == "independiente" || strings.Contains(tName, "avellaneda")) && !strings.Contains(tName, "rivadavia") { matchTeam = true }
			if (oName == "independiente" || strings.Contains(oName, "avellaneda")) && !strings.Contains(oName, "rivadavia") { matchTeam = true }
		} else if searchName == "racing" {
			// Desambiguación para Racing
			if (tName == "racing" || strings.Contains(tName, "club")) && !strings.Contains(tName, "cordoba") { matchTeam = true }
			if (oName == "racing" || strings.Contains(oName, "club")) && !strings.Contains(oName, "cordoba") { matchTeam = true }
		} else if strings.Contains(tName, searchName) || strings.Contains(oName, searchName) {
			matchTeam = true
		}

		if matchTeam {
			// Calculamos un score de prioridad
			// 1. Prioridad por Estado (LIVE = 0, FINAL/SCHEDULED TODAY = 10, OTHERS = 20)
			statusScore := 20
			if f.Status == "LIVE" { 
				statusScore = 0 
			} else if strings.Contains(f.Date, time.Now().Format("02/01")) {
				statusScore = 10
			}

			// 2. Prioridad por Torneo (Primera = 0, Otros = 100)
			tournScore := 100
			if strings.Contains(tourn, "liga profesional") || strings.Contains(tourn, "primera division") {
				tournScore = 0
			} else if strings.Contains(tourn, "copa argentina") || strings.Contains(tourn, "libertadores") || strings.Contains(tourn, "sudamericana") {
				tournScore = 50
			}

			totalScore := statusScore + tournScore
			if totalScore < lowestScore {
				lowestScore = totalScore
				bestMatch = f
				found = true
			}
		}
	}

	if !found {
		bestMatch = MatchData{
			Tournament: "Sin partidos agendados", Team: teamName, Opponent: "N/A", Date: "--/--", Time: "--:--", Stadium: "A confirmar", Status: "SCHEDULED",
		}
	} else {
		bestMatch.TeamCrest = GetCrestURL(bestMatch.Team)
		bestMatch.OpponentCrest = GetCrestURL(bestMatch.Opponent)
	}

	return UserSportsData{ Match: bestMatch, F1: all.F1, UFC: all.UFC }
}

var TeamMapping = map[string]string{
	"boca": "boca", "river": "river", "racing": "racing", "independiente": "independiente",
	"sanlorenzo": "sanlorenzo", "talleres": "talleres", "instituto": "instituto", "godoycruz": "godoycruz",
	"velez": "velez", "rosariocentral": "rosariocentral", "newells": "newells", "huracan": "huracan",
	"defensayjusticia": "defensayjusticia", "estudiantes": "estudiantes", "lanus": "lanus", "banfield": "banfield",
	"barracas": "barracascentral", "belgrano": "belgrano", "union": "union", "tigre": "tigre", "platense": "platense",
	"atleticotucuman": "atleticotucuman", "argentinos": "argentinosjuniors", "centralcordoba": "centralcordobasde",
	"independienterivadavia": "independienterivadavia", "riestra": "riestra", "sarmiento": "sarmiento", "gimnasia": "gimnasialp",
	"racingdecordoba": "afa",
	"estudiantesderiocuarto": "estudiantesrc",
	"estudiantesrc": "estudiantesrc",
}

func GetTeams() []string {
	return []string{"Boca Juniors", "River Plate", "Racing Club", "Independiente", "San Lorenzo", "Talleres", "Godoy Cruz", "Vélez Sarsfield", "Rosario Central", "Newell's Old Boys", "Estudiantes", "Gimnasia", "Huracán", "Lanús"}
}

func GetCrestURL(teamName string) string {
	name := normalizeName(teamName)
	
	// 1. Casos de desambiguación ultra-específicos
	if strings.Contains(name, "rivadavia") { return "/static/assets/crests/independienterivadavia.png" }
	if strings.Contains(name, "riocuarto") || strings.Contains(name, "estudiantesrc") { return "/static/assets/crests/estudiantesrc.png" }
	
	// Si dice Independiente seco o Avellaneda, es el Rojo
	if name == "independiente" || (strings.Contains(name, "independiente") && strings.Contains(name, "avellaneda")) {
		return "/static/assets/crests/independiente.png"
	}

	// 2. Búsqueda exacta en el mapeo (prioridad alta)
	if v, ok := TeamMapping[name]; ok {
		return "/static/assets/crests/" + v + ".png"
	}

	// 3. Búsqueda de coincidencia parcial como último recurso
	for k, v := range TeamMapping {
		if name == k { return "/static/assets/crests/" + v + ".png" }
	}
	
	// Si contiene la palabra clave y es un equipo de primera (búsqueda inversa)
	for k, v := range TeamMapping {
		if strings.Contains(name, k) {
			return "/static/assets/crests/" + v + ".png"
		}
	}

	return "/static/assets/crests/afa.png"
}

func GetCircuitData(circuitName string) string { return "/static/assets/circuits/" + circuitName + ".svg" }
func GetFlagURL(iso string) string { return "/static/assets/flags/" + iso + ".svg" }

func normalizeName(name string) string {
	n := strings.ToLower(name)
	// Eliminar acentos
	n = strings.ReplaceAll(n, "á", "a")
	n = strings.ReplaceAll(n, "é", "e")
	n = strings.ReplaceAll(n, "í", "i")
	n = strings.ReplaceAll(n, "ó", "o")
	n = strings.ReplaceAll(n, "ú", "u")
	// Eliminar caracteres especiales comunes en nombres de equipos
	n = strings.ReplaceAll(n, "'", "")
	n = strings.ReplaceAll(n, "’", "")
	n = strings.ReplaceAll(n, " ", "")
	n = strings.ReplaceAll(n, "(", "")
	n = strings.ReplaceAll(n, ")", "")
	n = strings.ReplaceAll(n, ".", "")
	n = strings.ReplaceAll(n, "-", "")
	return n
}

type ESPNScoreboard struct {
	Leagues []struct { Name string `json:"name"` } `json:"leagues"`
	Events []struct {
		Name      string `json:"name"`
		ShortName string `json:"shortName"`
		Date      string `json:"date"`
		Status    struct {
			Type struct { State string `json:"state"` } `json:"type"`
			DisplayClock string `json:"displayClock"`
		} `json:"status"`
		Competitions []struct {
			Venue struct { FullName string `json:"fullName"` } `json:"venue"`
			Notes []struct { Text string `json:"text"` } `json:"notes"`
			Status struct {
				Type struct { State string `json:"state"` } `json:"type"`
			} `json:"status"`
			Competitors []struct {
				HomeAway string `json:"homeAway"`
				Score    string `json:"score"`
				Team     struct { DisplayName string `json:"displayName"` } `json:"team"`
				Athlete  struct { DisplayName string `json:"displayName"` } `json:"athlete"`
			} `json:"competitors"`
			Broadcasts []struct { Names []string `json:"names"` } `json:"broadcasts"`
			GeoBroadcasts []struct {
				Type struct { ShortName string `json:"shortName"` } `json:"type"`
				Media struct { ShortName string `json:"shortName"` } `json:"media"`
			} `json:"geoBroadcasts"`
		} `json:"competitions"`
	} `json:"events"`
}

func parseToArgentina(dateStr string) (time.Time, error) {
	loc, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil { loc = time.FixedZone("ART", -3*60*60) }
	layouts := []string{time.RFC3339, "2006-01-02T15:04Z", "2006-01-02T15:04:05Z"}
	for _, layout := range layouts { if t, err := time.Parse(layout, dateStr); err == nil { return t.In(loc), nil } }
	return time.Time{}, fmt.Errorf("err")
}

func guessBroadcaster(tournament string) string {
	t := strings.ToLower(tournament)
	if strings.Contains(t, "liga profesional") || strings.Contains(t, "primera división") || strings.Contains(t, "copa de la liga") { 
		return "ESPN Premium / TNT Sports" 
	}
	if strings.Contains(t, "copa argentina") { return "TyC Sports" }
	if strings.Contains(t, "libertadores") || strings.Contains(t, "sudamericana") { return "ESPN / Fox Sports" }
	if strings.Contains(t, "nacional") { return "TyC Sports / DSports" }
	return "A confirmar"
}

func fetchLiveMatches(url string) []MatchData {
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 { return nil }
	defer resp.Body.Close()
	var sb ESPNScoreboard
	if err := json.NewDecoder(resp.Body).Decode(&sb); err != nil { return nil }
	var liveMatches []MatchData
	tournamentName := "Torneo Local"
	if len(sb.Leagues) > 0 { tournamentName = sb.Leagues[0].Name }
	for _, event := range sb.Events {
		if len(event.Competitions) == 0 { continue }
		comp := event.Competitions[0]
		var home, away, hScore, aScore string
		for _, c := range comp.Competitors { if c.HomeAway == "home" { home = c.Team.DisplayName; hScore = c.Score } else { away = c.Team.DisplayName; aScore = c.Score } }
		status := "SCHEDULED"
		if event.Status.Type.State == "in" { status = "LIVE" } else if event.Status.Type.State == "post" { status = "FINAL" }
		dateStr, timeStr := "", ""
		if t, err := parseToArgentina(event.Date); err == nil {
			days := map[time.Weekday]string{time.Sunday: "Dom", time.Monday: "Lun", time.Tuesday: "Mar", time.Wednesday: "Mié", time.Thursday: "Jue", time.Friday: "Vie", time.Saturday: "Sáb"}
			dateStr = days[t.Weekday()] + " " + t.Format("02/01"); timeStr = t.Format("15:04")
		}
		
		channel := ""
		// 1. Intentar con Broadcasts generales
		for _, b := range comp.Broadcasts {
			if len(b.Names) > 0 {
				channel = b.Names[0]
				break
			}
		}
		
		// 2. Intentar con GeoBroadcasts (más preciso para región)
		if channel == "" || channel == "A confirmar" {
			for _, gb := range comp.GeoBroadcasts {
				if gb.Media.ShortName != "" {
					channel = gb.Media.ShortName
					break
				}
			}
		}

		// 3. Buscar en las Notas del evento (a veces dice "TV: Canal")
		if channel == "" || channel == "A confirmar" {
			for _, note := range comp.Notes {
				if strings.Contains(strings.ToLower(note.Text), "tv:") {
					parts := strings.Split(note.Text, ":")
					if len(parts) > 1 {
						channel = strings.TrimSpace(parts[1])
						break
					}
				}
			}
		}
		
		if channel == "" || channel == "A confirmar" {
			channel = guessBroadcaster(tournamentName)
		}

		// Limpieza de nombres comunes
		channel = strings.ReplaceAll(channel, "ESP+", "ESPN Premium")
		channel = strings.ReplaceAll(channel, "TNTS", "TNT Sports")

		liveMatches = append(liveMatches, MatchData{
			Team: home, Opponent: away, Date: dateStr, Time: timeStr, Tournament: tournamentName,
			Stadium: comp.Venue.FullName, Channel: channel, HomeScore: hScore, AwayScore: aScore, Status: status, Clock: event.Status.DisplayClock,
		})
	}
	return liveMatches
}

func fetchLiveF1() F1Race {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.jolpi.ca/ergast/f1/current/next.json")
	if err != nil || resp.StatusCode != 200 { return F1Race{GrandPrix: "Sin carreras"} }
	defer resp.Body.Close()
	var data struct { MRData struct { RaceTable struct { Races []struct { RaceName string `json:"raceName"`; Circuit struct { CircuitId string `json:"circuitId"`; Location struct { Country string `json:"country"`; Lat string `json:"lat"`; Long string `json:"long"` } `json:"location"` } `json:"Circuit"`; Date string `json:"date"`; Time string `json:"time"`; FirstPractice struct { Date string `json:"date"`; Time string `json:"time"` } `json:"FirstPractice"`; SecondPractice struct { Date string `json:"date"`; Time string `json:"time"` } `json:"SecondPractice"`; ThirdPractice struct { Date string `json:"date"`; Time string `json:"time"` } `json:"ThirdPractice"`; Qualifying struct { Date string `json:"date"`; Time string `json:"time"` } `json:"Qualifying"` } `json:"Races"` } `json:"RaceTable"` } `json:"MRData"` }
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil || len(data.MRData.RaceTable.Races) == 0 { return F1Race{GrandPrix: "Sin carreras"} }
	race := data.MRData.RaceTable.Races[0]

	// --- FETCH WEATHER FOR CIRCUIT ---
	weatherMap := make(map[string]string)
	if race.Circuit.Location.Lat != "" && race.Circuit.Location.Long != "" {
		wUrl := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%s&longitude=%s&daily=weather_code&timezone=auto", race.Circuit.Location.Lat, race.Circuit.Location.Long)
		if wResp, wErr := client.Get(wUrl); wErr == nil && wResp.StatusCode == 200 {
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
					if code > 0 && code <= 3 { icon = "cloud" }
					if code >= 45 && code <= 48 { icon = "cloud-fog" }
					if code >= 51 && code <= 67 { icon = "cloud-rain" }
					if code >= 71 && code <= 77 { icon = "snowflake" }
					if code >= 80 && code <= 82 { icon = "cloud-rain" }
					if code >= 95 { icon = "cloud-lightning" }
					weatherMap[t] = icon
				}
			}
			wResp.Body.Close()
		}
	}
	// ---------------------------------

	formatDate := func(d, t string) (string, string) {
		if d == "" || t == "" { return "", "" }
		if p, err := parseToArgentina(d + "T" + t); err == nil {
			days := map[time.Weekday]string{time.Sunday: "Dom", time.Monday: "Lun", time.Tuesday: "Mar", time.Wednesday: "Mié", time.Thursday: "Jue", time.Friday: "Vie", time.Saturday: "Sáb"}
			return days[p.Weekday()] + " " + p.Format("02/01"), p.Format("15:04")
		}
		return d, t
	}
	fp1D, fp1T := formatDate(race.FirstPractice.Date, race.FirstPractice.Time)
	fp2D, fp2T := formatDate(race.SecondPractice.Date, race.SecondPractice.Time)
	fp3D, fp3T := formatDate(race.ThirdPractice.Date, race.ThirdPractice.Time)
	qD, qT := formatDate(race.Qualifying.Date, race.Qualifying.Time)
	rD, rT := formatDate(race.Date, race.Time)
	sessions := []F1Session{}
	if fp1D != "" { sessions = append(sessions, F1Session{Name: "FP1", Date: fp1D, Time: fp1T, WeatherIcon: weatherMap[race.FirstPractice.Date]}) }
	if fp2D != "" { sessions = append(sessions, F1Session{Name: "FP2", Date: fp2D, Time: fp2T, WeatherIcon: weatherMap[race.SecondPractice.Date]}) }
	if fp3D != "" { sessions = append(sessions, F1Session{Name: "FP3", Date: fp3D, Time: fp3T, WeatherIcon: weatherMap[race.ThirdPractice.Date]}) }
	if qD != "" { sessions = append(sessions, F1Session{Name: "Qualy", Date: qD, Time: qT, WeatherIcon: weatherMap[race.Qualifying.Date]}) }
	if rD != "" { sessions = append(sessions, F1Session{Name: "Race", Date: rD, Time: rT, WeatherIcon: weatherMap[race.Date]}) }
	return F1Race{ GrandPrix: race.RaceName, Circuit: race.Circuit.CircuitId, FlagURL: GetFlagURL("un"), PosterURL: GetCircuitData(race.Circuit.CircuitId), Sessions: sessions }
}

func fetchLiveUFC() UFCMatch {
	client := &http.Client{Timeout: 5 * time.Second}
	
	// Calculamos rango de fechas: desde hace 2 días hasta dentro de 30 días
	// Esto nos asegura capturar el evento recién terminado (si lo hay) y los próximos.
	now := time.Now()
	startDate := now.AddDate(0, 0, -2).Format("20060102")
	endDate := now.AddDate(0, 0, 30).Format("20060102")
	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/mma/ufc/scoreboard?dates=%s-%s", startDate, endDate)
	
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != 200 { return UFCMatch{EventName: "Sin eventos"} }
	defer resp.Body.Close()
	
	var sb ESPNScoreboard
	if err := json.NewDecoder(resp.Body).Decode(&sb); err != nil || len(sb.Events) == 0 { return UFCMatch{EventName: "Sin eventos"} }
	
	// Buscar el primer evento que NO haya terminado.
	// Si todos terminaron, tomamos el último (el más reciente).
	eventIdx := -1
	for i, e := range sb.Events {
		if e.Status.Type.State != "post" {
			eventIdx = i
			break
		}
	}
	if eventIdx == -1 {
		eventIdx = len(sb.Events) - 1
	}
	mainEvent := sb.Events[eventIdx]

	var mainCard []string
	// Recorremos las peleas (competitions) del evento seleccionado
	for i := len(mainEvent.Competitions) - 1; i >= 0; i-- {
		comp := mainEvent.Competitions[i]
		if len(comp.Competitors) >= 2 {
			p1 := comp.Competitors[0].Athlete.DisplayName; if p1 == "" { p1 = comp.Competitors[0].Team.DisplayName }
			p2 := comp.Competitors[1].Athlete.DisplayName; if p2 == "" { p2 = comp.Competitors[1].Team.DisplayName }
			
			if p1 != "" && p2 != "" {
				fightStatus := ""
				// Si la pelea individual está en vivo o terminada, lo marcamos
				if comp.Status.Type.State == "in" { fightStatus = " (EN VIVO)" }
				if comp.Status.Type.State == "post" { fightStatus = " (FINAL)" }
				
				mainCard = append(mainCard, p1 + " vs " + p2 + fightStatus)
			}
		}
		if len(mainCard) >= 5 { break }
	}

	dateStr, timeStr := "--/--", "--:--"
	if t, err := parseToArgentina(mainEvent.Date); err == nil {
		days := map[time.Weekday]string{time.Sunday: "Dom", time.Monday: "Lun", time.Tuesday: "Mar", time.Wednesday: "Mié", time.Thursday: "Jue", time.Friday: "Vie", time.Saturday: "Sáb"}
		dateStr = days[t.Weekday()] + " " + t.Format("02/01"); timeStr = t.Format("15:04")
	}

	status := "SCHEDULED"
	if mainEvent.Status.Type.State == "in" { status = "LIVE" } else if mainEvent.Status.Type.State == "post" { status = "FINAL" }

	name := mainEvent.Name; if name == "" { name = mainEvent.ShortName }
	return UFCMatch{ EventName: name, Date: dateStr, Time: timeStr, Status: status, MainCard: mainCard }
}

func GetSportsData() SportsData {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	return cachedData
}

type PromiedosMatch struct {
	Channel string
	HScore  string
	AScore  string
	Status  string
	Clock   string
}

func fetchPromiedosChannels() map[string]PromiedosMatch {
	client := &http.Client{Timeout: 7 * time.Second}
	req, _ := http.NewRequest("GET", "https://www.promiedos.com.ar/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[PROMIEDOS] Error de red: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	startMarker := "<script id=\"__NEXT_DATA__\" type=\"application/json\">"
	startIdx := strings.Index(html, startMarker)
	if startIdx == -1 {
		log.Printf("[PROMIEDOS] No se encontró __NEXT_DATA__\n")
		return nil
	}
	startIdx += len(startMarker)
	endIdx := strings.Index(html[startIdx:], "</script>")
	if endIdx == -1 { return nil }
	
	jsonStr := html[startIdx : startIdx+endIdx]
	
	type NextData struct {
		Props struct {
			PageProps struct {
				Data struct {
					Leagues []struct {
						Games []struct {
							Teams []struct { Name string `json:"name"` } `json:"teams"`
							Scores []interface{} `json:"scores"`
							TvNetworks []struct { Name string `json:"name"` } `json:"tv_networks"`
							Status struct { 
								Name string `json:"name"`
								Enum int `json:"enum"`
							} `json:"status"`
							GameTime int `json:"game_time"`
						} `json:"games"`
					} `json:"leagues"`
				} `json:"data"`
			} `json:"pageProps"`
		} `json:"props"`
	}

	var data NextData
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		log.Printf("[PROMIEDOS] Error unmarshal: %v\n", err)
		return nil
	}

	matches := make(map[string]PromiedosMatch)
	for _, league := range data.Props.PageProps.Data.Leagues {
		for _, game := range league.Games {
			if len(game.Teams) >= 2 {
				ch := ""
				if len(game.TvNetworks) > 0 { ch = game.TvNetworks[0].Name }
				
				hScore, aScore := "", ""
				if len(game.Scores) >= 2 {
					hScore = fmt.Sprintf("%v", game.Scores[0])
					aScore = fmt.Sprintf("%v", game.Scores[1])
				}

				status := "SCHEDULED"
				if game.Status.Enum == 2 { status = "LIVE" } else if game.Status.Enum == 3 { status = "FINAL" }
				
				pm := PromiedosMatch{
					Channel: ch,
					HScore:  hScore,
					AScore:  aScore,
					Status:  status,
					Clock:   fmt.Sprintf("%d'", game.GameTime),
				}
				
				hName := normalizeName(game.Teams[0].Name)
				aName := normalizeName(game.Teams[1].Name)
				matches[hName] = pm
				matches[aName] = pm
			}
		}
	}
	log.Printf("[PROMIEDOS] Update finalizado. Partidos procesados: %d\n", len(matches))
	return matches
}

func fetchFreshSportsData() SportsData {
	f1Data := fetchLiveF1()
	ufcData := fetchLiveUFC()
	
	// Fuente de verdad para Argentina: Promiedos
	promiedosMap := fetchPromiedosChannels()
	
	var allMatches []MatchData
	now := time.Now()
	later := now.AddDate(0, 0, 15)
	dateRange := now.Format("20060102") + "-" + later.Format("20060102")
	urls := []string{
		"https://site.api.espn.com/apis/site/v2/sports/soccer/arg.1/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
		"https://site.api.espn.com/apis/site/v2/sports/soccer/arg.2/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
		"https://site.api.espn.com/apis/site/v2/sports/soccer/arg.copa/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
		"https://site.api.espn.com/apis/site/v2/sports/soccer/lib/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
		"https://site.api.espn.com/apis/site/v2/sports/soccer/sud.copa/scoreboard?lang=es&region=ar&limit=50&dates=" + dateRange,
	}
	for _, url := range urls {
		if m := fetchLiveMatches(url); m != nil { 
			for i := range m {
				pTeam := normalizeName(m[i].Team)
				pOpp := normalizeName(m[i].Opponent)
				
				// Sobrescribir TODO con Promiedos si hay coincidencia
				found := false
				for key, val := range promiedosMap {
					if (len(key) > 3 && (strings.Contains(pTeam, key) || strings.Contains(key, pTeam))) ||
					   (len(key) > 3 && (strings.Contains(pOpp, key) || strings.Contains(key, pOpp))) {
						if val.Channel != "" { m[i].Channel = val.Channel }
						if val.HScore != "" { m[i].HomeScore = val.HScore }
						if val.AScore != "" { m[i].AwayScore = val.AScore }
						if val.Status != "" { m[i].Status = val.Status }
						if val.Clock != "" && val.Clock != "0'" { m[i].Clock = val.Clock }
						found = true
						break
					}
				}
				if found {
					// Pequeña corrección de nombres de canales tras el merge
					m[i].Channel = strings.ReplaceAll(m[i].Channel, "Premium", "")
					m[i].Channel = strings.TrimSpace(m[i].Channel)
				}
			}
			allMatches = append(allMatches, m...) 
		}
	}
	return SportsData{ AllMatches: allMatches, F1: f1Data, UFC: ufcData }
}
