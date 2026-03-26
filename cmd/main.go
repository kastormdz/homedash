package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"homedash/internal/crypto"
	"homedash/internal/earthquake"
	"homedash/internal/finance"
	"homedash/internal/holidays"
	"homedash/internal/sports"
	"homedash/internal/weather"
)

var (
	tmpls        *template.Template
	lastBTCPrice float64
	btcPriceLock sync.Mutex
)

type AppSettings struct {
	Team         string `json:"team"`
	City         string `json:"city"`
	Province     string `json:"province"`
	Lat          string `json:"lat"`
	Lon          string `json:"lon"`
	ShowFootball bool   `json:"showFootball"`
	ShowF1       bool   `json:"showF1"`
	ShowUFC      bool   `json:"showUFC"`
	ShowFinance  bool   `json:"showFinance"`
	Theme        string `json:"theme"`
}

func getDefaultSettings() AppSettings {
	return AppSettings{
		Team:         "Boca",
		City:         "Mendoza, AR",
		Province:     "Mendoza",
		Lat:          "-32.89",
		Lon:          "-68.82",
		ShowFootball: true,
		ShowF1:       true,
		ShowUFC:      true,
		ShowFinance:  true,
		Theme:        "dark",
	}
}

func getSettings(r *http.Request) AppSettings {
	cookie, err := r.Cookie("settings")
	if err == nil {
		val, _ := url.QueryUnescape(cookie.Value)
		s := getDefaultSettings()
		if err := json.Unmarshal([]byte(val), &s); err == nil {
			// Si falta la provincia (ej. cookies viejas), intentamos recuperarla
			if s.Province == "" && s.City != "" {
				// SearchCity usa caché interna, así que no es costoso si se repite
				_, _, _, province, err := weather.SearchCity(s.City)
				if err == nil {
					s.Province = province
				}
			}
			return s
		}
	}
	return getDefaultSettings()
}

func init() {
	var err error
	tmpls, err = template.ParseGlob(filepath.Join("templates", "*.html"))
	if err != nil {
		log.Fatalf("Error cargando plantillas: %v", err)
	}
}

func main() {
	log.Println("Iniciando Homedash...")

	// Sincronización inicial con reintentos (Previene arranque vacío en Docker)
	maxRetries := 5
	retryDelay := 5 * time.Second

	log.Println("[INIT] Cargando datos iniciales...")

	// 1. Deportes (Bloqueante hasta éxito o max retries)
	for i := 0; i < maxRetries; i++ {
		err := sports.ForceUpdate()
		if err == nil {
			log.Println("[INIT] Deportes cargados con éxito.")
			break
		}
		log.Printf("[INIT] Error cargando deportes (intento %d/%d): %v. Reintentando en %v...\n", i+1, maxRetries, err, retryDelay)
		time.Sleep(retryDelay)
	}

	// 2. Crypto y Finanzas (Intentar una vez rápido, si fallan no bloqueamos el inicio pero los lanzamos)
	if err := crypto.UpdateBTC(); err != nil {
		log.Printf("[INIT] Aviso: BTC no se pudo cargar inicialmente: %v\n", err)
	}
	if err := finance.UpdateDollar(); err != nil {
		log.Printf("[INIT] Aviso: Finanzas no se pudieron cargar inicialmente: %v\n", err)
	}

	// Iniciar bucles de actualización en segundo plano
	sports.StartUpdateLoop()
	crypto.StartUpdateLoop()
	finance.StartUpdateLoop()

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/weather", handleWeather)
	http.HandleFunc("/crest", handleCrestProxy)
	http.HandleFunc("/settings", handleSettings)
	http.HandleFunc("/autocomplete/teams", handleAutocompleteTeams)
	http.HandleFunc("/autocomplete/cities", handleAutocompleteCities)

	port := ":8060"
	log.Printf("Servidor corriendo en http://localhost%s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	settings := getSettings(r)
	now := time.Now()
	data := struct {
		Holiday          *holidays.Holiday
		UpcomingHolidays []holidays.UpcomingHoliday
		ShowF1           bool
		ShowFootball     bool
		ShowUFC          bool
		ShowFinance      bool
		Theme            string
	}{
		Holiday:          holidays.GetHolidayToday(now),
		UpcomingHolidays: holidays.GetUpcomingHolidays(now),
		ShowF1:           settings.ShowF1,
		ShowFootball:     settings.ShowFootball,
		ShowUFC:          settings.ShowUFC,
		ShowFinance:      settings.ShowFinance,
		Theme:            settings.Theme,
	}
	err := tmpls.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleWeather(w http.ResponseWriter, r *http.Request) {
	settings := getSettings(r)

	weatherData, _ := weather.GetWeather(settings.Lat, settings.Lon)
	btcPrice := crypto.GetCachedBTC()

	btcPriceLock.Lock()
	defer btcPriceLock.Unlock()
	trend := 0
	if lastBTCPrice > 0 && btcPrice > 0 {
		if btcPrice > lastBTCPrice {
			trend = 1
		} else if btcPrice < lastBTCPrice {
			trend = -1
		}
	}
	if btcPrice > 0 {
		lastBTCPrice = btcPrice
	}

	rainProb := 0
	var current weather.CurrentWeather
	var forecast []weather.ForecastItem
	var sunrise, sunset string
	aqi := 0
	aqiDesc := "Desconocido"
	moonIcon := "moon"
	moonPhase := "---"

	if weatherData != nil {
		current = weatherData.Current
		forecast = weatherData.GetForecastList()
		sunrise = weather.FormatTime(weatherData.Daily.Sunrise[0])
		sunset = weather.FormatTime(weatherData.Daily.Sunset[0])
		if len(weatherData.Daily.RainProb) > 0 {
			rainProb = weatherData.Daily.RainProb[0]
		}
		aqi = weatherData.AQI
		aqiDesc = weatherData.AQIDesc
		moonIcon = weatherData.MoonIcon
		moonPhase = weatherData.MoonPhaseName
	}

	finData := finance.GetCachedFinance()

	viewModel := struct {
		weather.WeatherViewModel
		BTCPrice    float64
		BTCTrend    int
		RainProb    int
		Earthquakes []earthquake.EarthquakeData
		Alert       weather.WeatherAlert
	}{
		WeatherViewModel: weather.WeatherViewModel{
			Current:       current,
			Forecast:      forecast,
			Sunrise:       sunrise,
			Sunset:        sunset,
			NextMatch:     sports.GetSportsDataForUser(settings.Team),
			City:          settings.City,
			ShowF1:        settings.ShowF1,
			ShowFootball:  settings.ShowFootball,
			ShowUFC:       settings.ShowUFC,
			ShowFinance:   settings.ShowFinance,
			Dolar:         finData,
			BTCChange:     crypto.GetCachedBTCChange(),
			AQI:           aqi,
			AQIDesc:       aqiDesc,
			MoonIcon:      moonIcon,
			MoonPhaseName: moonPhase,
		},
		BTCPrice:    btcPrice,
		BTCTrend:    trend,
		RainProb:    rainProb,
		Earthquakes: earthquake.GetLatestEarthquakes(),
		Alert:       weather.GetSMNAlert(settings.Province, settings.City),
	}

	err := tmpls.ExecuteTemplate(w, "weather.html", viewModel)
	if err != nil {
		log.Printf("Error renderizando weather: %v", err)
	}
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s := getSettings(r)

		team := r.FormValue("team")
		if team != "" {
			s.Team = team
		}

		s.ShowF1 = r.FormValue("showF1") == "on"
		s.ShowFootball = r.FormValue("showFootball") == "on"
		s.ShowUFC = r.FormValue("showUFC") == "on"
		s.ShowFinance = r.FormValue("showFinance") == "on"

		theme := r.FormValue("theme")
		if theme != "" {
			s.Theme = theme
		}

		city := r.FormValue("city")
		if city != "" {
			name, lat, lon, province, err := weather.SearchCity(city)
			if err == nil {
				s.City = name
				s.Lat = lat
				s.Lon = lon
				s.Province = province
			}
		}

		// Guardar en Cookie (1 año)
		val, _ := json.Marshal(s)
		http.SetCookie(w, &http.Cookie{
			Name:     "settings",
			Value:    url.QueryEscape(string(val)),
			Path:     "/",
			Expires:  time.Now().AddDate(1, 0, 0),
			HttpOnly: true,
		})

		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
}

func handleCrestProxy(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	name := r.URL.Query().Get("name")
	if targetURL == "" {
		http.Error(w, "Falta URL", http.StatusBadRequest)
		return
	}

	ext := ".png"
	if strings.Contains(targetURL, ".svg") {
		ext = ".svg"
	}

	if name != "" {
		localPath := filepath.Join("static", "assets", "cache", name+ext)
		if info, err := os.Stat(localPath); err == nil && info.Size() > 500 {
			http.ServeFile(w, r, localPath)
			return
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("GET", targetURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		http.Error(w, "Error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "public, max-age=604800")

	if name != "" {
		localPath := filepath.Join("static", "assets", "cache", name+ext)
		_ = os.MkdirAll(filepath.Dir(localPath), 0755)
		out, _ := os.Create(localPath)
		if out != nil {
			multi := io.MultiWriter(w, out)
			_, _ = io.Copy(multi, resp.Body)
			out.Close()
			return
		}
	}
	_, _ = io.Copy(w, resp.Body)
}

func handleAutocompleteTeams(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(r.URL.Query().Get("team"))
	teams := sports.GetTeams()
	var matches []string
	for _, t := range teams {
		if strings.Contains(strings.ToLower(t), q) {
			matches = append(matches, t)
		}
	}
	w.Header().Set("Content-Type", "text/html")
	for _, m := range matches {
		fmt.Fprintf(w, "<option value=\"%s\">\n", m)
	}
}

func handleAutocompleteCities(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("city")
	if len(q) < 3 {
		return
	}
	name, _, _, _, err := weather.SearchCity(q)
	if err == nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<option value=\"%s\">\n", name)
	}
}
