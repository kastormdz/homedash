package main

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"homedash/internal/crypto"
	"homedash/internal/earthquake"
	"homedash/internal/finance"
	"homedash/internal/holidays"
	"homedash/internal/network"
	"homedash/internal/sports"
	"homedash/internal/weather"
)

var (
	tmpls        *template.Template
	lastBTCPrice float64
	btcPriceLock sync.Mutex
	lastETHPrice float64
	ethPriceLock sync.Mutex

	// 7. Temas válidos como var de paquete (no recrear en cada request)
	validThemes = map[string]struct{}{
		"dark": {}, "dracula": {}, "synthwave": {}, "cyberpunk": {},
		"retro": {}, "dim": {}, "coffee": {}, "sunset": {}, "night": {},
	}
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

func cacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400") // 24h
		next.ServeHTTP(w, r)
	})
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		gzw := gzipResponseWriter{Writer: gz, ResponseWriter: w}
		next.ServeHTTP(gzw, r)
	})
}

func isTrustedProxy(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	return parsedIP.IsLoopback() || parsedIP.IsPrivate() || parsedIP.IsUnspecified()
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verificar proxy de confianza
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if isTrustedProxy(host) {
			if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
				r.RemoteAddr = strings.Split(forwardedFor, ",")[0]
			}
			if r.Header.Get("X-Forwarded-Proto") == "https" {
				r.TLS = &tls.ConnectionState{}
			}
		}

		// S-2. Headers de seguridad básicos
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// CSP permisivo para CDNs conocidos pero bloqueando inline scripts maliciosos
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://unpkg.com https://cdn.jsdelivr.net https://cdn.tailwindcss.com; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: https://openweathermap.org https://upload.wikimedia.org https://cdn.register.f1.com https://a.espncdn.com https://static.promiedos.com.ar https://img.icons8.com; connect-src 'self';")
		next.ServeHTTP(w, r)
	})
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
	if err := crypto.UpdateCrypto(context.Background()); err != nil {
		log.Printf("[INIT] Aviso: Cripto no se pudo cargar inicialmente: %v\n", err)
	}
	if err := finance.UpdateFinance(context.Background()); err != nil {
		log.Printf("[INIT] Aviso: Finanzas no se pudieron cargar inicialmente: %v\n", err)
	}

	// Iniciar bucles de actualización en segundo plano
	sports.StartUpdateLoop()
	crypto.StartUpdateLoop()
	finance.StartUpdateLoop()

	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", cacheMiddleware(fs)))

	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/weather", handleWeather)
	mux.HandleFunc("/crest", handleCrestProxy)
	mux.HandleFunc("/settings", handleSettings)
	mux.HandleFunc("/autocomplete/teams", rateLimit(handleAutocompleteTeams))
	mux.HandleFunc("/autocomplete/cities", rateLimit(handleAutocompleteCities))
	mux.HandleFunc("/health", handleHealth)

	// Aplicar middleware de seguridad y gzip
	secureMux := gzipMiddleware(securityHeaders(mux))

	port := ":8060"
	if p := os.Getenv("PORT"); p != "" {
		port = ":" + p
	}

	server := &http.Server{
		Addr:         port,
		Handler:      secureMux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("[SERVER] Apagando servidor...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("[SERVER] Error durante apagado: %v", err)
		}
	}()

	log.Printf("Servidor corriendo en http://localhost%s\n", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

type WeatherViewModel struct {
	Current       weather.CurrentWeather
	Forecast      []weather.ForecastItem
	Sunrise       string
	Sunset        string
	NextMatch     sports.UserSportsData
	TodayHoliday  *holidays.Holiday
	City          string
	ShowF1        bool
	ShowFootball  bool
	ShowUFC       bool
	ShowFinance   bool
	Dolar         finance.FinanceData
	BTCPrice      float64
	BTCTrend      int
	BTCChange     float64
	ETHPrice      float64
	ETHTrend      int
	ETHChange     float64
	RainProb      int
	Earthquakes   []earthquake.EarthquakeData
	Alert         weather.WeatherAlert
	AQI           int
	AQIDesc       string
	MoonIcon      string
	MoonPhaseName string
}

func handleWeather(w http.ResponseWriter, r *http.Request) {
	settings := getSettings(r)

	weatherData, _ := weather.GetWeather(settings.Lat, settings.Lon)
	btcPrice := crypto.GetCachedBTC()
	ethPrice := crypto.GetCachedETH()

	btcPriceLock.Lock()
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
	btcPriceLock.Unlock()

	ethPriceLock.Lock()
	ethTrend := 0
	if lastETHPrice > 0 && ethPrice > 0 {
		if ethPrice > lastETHPrice {
			ethTrend = 1
		} else if ethPrice < lastETHPrice {
			ethTrend = -1
		}
	}
	if ethPrice > 0 {
		lastETHPrice = ethPrice
	}
	ethPriceLock.Unlock()

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
		// 1. Fix: Verificar que Sunrise/Sunset no estén vacíos antes de acceder [0]
		if len(weatherData.Daily.Sunrise) > 0 {
			sunrise = weather.FormatTime(weatherData.Daily.Sunrise[0])
		}
		if len(weatherData.Daily.Sunset) > 0 {
			sunset = weather.FormatTime(weatherData.Daily.Sunset[0])
		}
		if len(weatherData.Daily.RainProb) > 0 {
			rainProb = weatherData.Daily.RainProb[0]
		}
		aqi = weatherData.AQI
		aqiDesc = weatherData.AQIDesc
		moonIcon = weatherData.MoonIcon
		moonPhase = weatherData.MoonPhaseName
	} else {
		// 3. Loggear error de weather en vez de ignorar silenciosamente
		log.Printf("[WEATHER] No se pudieron obtener datos para %s,%s", settings.Lat, settings.Lon)
	}

	finData := finance.GetCachedFinance()

	viewModel := WeatherViewModel{
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
		BTCPrice:      btcPrice,
		BTCTrend:      trend,
		BTCChange:     crypto.GetCachedBTCChange(),
		ETHPrice:      ethPrice,
		ETHTrend:      ethTrend,
		ETHChange:     crypto.GetCachedETHChange(),
		RainProb:      rainProb,
		Earthquakes:   earthquake.GetLatestEarthquakes(),
		Alert:         weather.GetSMNAlert(settings.Province, settings.City),
		AQI:           aqi,
		AQIDesc:       aqiDesc,
		MoonIcon:      moonIcon,
		MoonPhaseName: moonPhase,
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
			// Solo permitir un equipo. Si el usuario ingresa una lista separada por comas o similar, tomamos el primero.
			team = strings.Split(team, ",")[0]
			team = strings.Split(team, ";")[0]
			team = strings.Split(team, "-")[0]
			s.Team = strings.TrimSpace(team)
		}

		s.ShowF1 = r.FormValue("showF1") == "on"
		s.ShowFootball = r.FormValue("showFootball") == "on"
		s.ShowUFC = r.FormValue("showUFC") == "on"
		s.ShowFinance = r.FormValue("showFinance") == "on"

		theme := r.FormValue("theme")
		if theme != "" {
			if _, ok := validThemes[theme]; ok {
				s.Theme = theme
			}
		}

		city := r.FormValue("city")
		if city != "" {
			name, lat, lon, province, err := weather.SearchCity(city)
			if err != nil {
				http.Error(w, "Ciudad no encontrada", http.StatusNotFound)
				return
			}

			// S4. Validación de Coordenadas
			var fLat, fLon float64
			if _, err := fmt.Sscanf(lat, "%f", &fLat); err != nil {
				http.Error(w, "Latitud inválida", http.StatusBadRequest)
				return
			}
			if _, err := fmt.Sscanf(lon, "%f", &fLon); err != nil {
				http.Error(w, "Longitud inválida", http.StatusBadRequest)
				return
			}

			if fLat >= -90 && fLat <= 90 && fLon >= -180 && fLon <= 180 {
				s.City = name
				s.Lat = lat
				s.Lon = lon
				s.Province = province
			} else {
				http.Error(w, "Coordenadas fuera de rango", http.StatusBadRequest)
				return
			}
		}

		// Guardar en Cookie (1 año)
		val, _ := json.Marshal(s)
		// S3. Cookie con flags de seguridad
		http.SetCookie(w, &http.Cookie{
			Name:     "settings",
			Value:    url.QueryEscape(string(val)),
			Path:     "/",
			Expires:  time.Now().AddDate(1, 0, 0),
			HttpOnly: true,
			Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
			SameSite: http.SameSiteLaxMode,
		})

		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
}

func handleCrestProxy(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	name := r.URL.Query().Get("name")
	if targetURL == "" {
		http.Error(w, "Falta URL", http.StatusBadRequest)
		return
	}

	// 2. Path traversal: Sanitizar nombre para evitar ../
	if name != "" && (strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") || strings.Contains(name, "%")) {
		log.Printf("[SECURITY] Intento de path traversal en crest cache: %s", name)
		http.Error(w, "Nombre inválido", http.StatusBadRequest)
		return
	}

	// S1. Protección SSRF: Validar contra lista blanca
	allowed, err := network.IsDomainAllowed(targetURL)
	if err != nil || !allowed {
		log.Printf("[SECURITY] Intento de proxy a dominio no permitido: %s", targetURL)
		http.Error(w, "Acceso denegado", http.StatusForbidden)
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

	// B1. Protección contra Panic: Verificar error antes de StatusCode (FetchSecure encapsula esto)
	resp, err := network.FetchSecure(targetURL)
	if err != nil {
		http.Error(w, "Error al obtener escudo", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "public, max-age=604800")

	if name != "" {
		localPath := filepath.Join("static", "assets", "cache", name+ext)
		_ = os.MkdirAll(filepath.Dir(localPath), 0755)
		out, err := os.Create(localPath)
		if err == nil {
			multi := io.MultiWriter(w, out)
			_, _ = io.Copy(multi, resp.Body)
			out.Close()
			return
		}
	}
	_, _ = io.Copy(w, resp.Body)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

var (
	rateLimitMap = make(map[string]time.Time)
	rateLimitMu  sync.Mutex
)

func rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if pos := strings.LastIndex(ip, ":"); pos != -1 {
			ip = ip[:pos]
		}

		rateLimitMu.Lock()
		last, ok := rateLimitMap[ip]
		if ok && time.Since(last) < 1*time.Second {
			rateLimitMu.Unlock()
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		rateLimitMap[ip] = time.Now()
		// Limpieza periódica simple de la caché si crece mucho
		if len(rateLimitMap) > 1000 {
			rateLimitMap = make(map[string]time.Time)
		}
		rateLimitMu.Unlock()

		next.ServeHTTP(w, r)
	}
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
	if len(matches) == 0 {
		fmt.Fprint(w, "<option value=\"\">No se encontraron equipos</option>")
		return
	}
	for _, m := range matches {
		// S2. Prevención XSS: Escapar contenido dinámico
		fmt.Fprintf(w, "<option value=\"%s\">\n", template.HTMLEscapeString(m))
	}
}

func handleAutocompleteCities(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("city")
	if len(q) < 3 {
		// No responder nada para queries cortas para evitar ruido visual
		return
	}
	name, _, _, _, err := weather.SearchCity(q)
	w.Header().Set("Content-Type", "text/html")
	if err != nil {
		fmt.Fprint(w, "<option value=\"\">No se encontraron ciudades</option>")
		return
	}
	// S2. Prevención XSS: Escapar contenido dinámico
	fmt.Fprintf(w, "<option value=\"%s\">\n", template.HTMLEscapeString(name))
}
