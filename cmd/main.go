package main

import (
	"bytes"
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
	tmpls     *template.Template
	tmplsLock sync.RWMutex

	// 7. Temas válidos como var de paquete (no recrear en cada request)
	validThemes = map[string]struct{}{
		"dark": {}, "dracula": {}, "synthwave": {}, "cyberpunk": {},
		"retro": {}, "dim": {}, "coffee": {}, "sunset": {}, "night": {},
		"nothing": {},
	}
)

func loadTemplates() {
	t := template.New("").Funcs(template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
		"contains": func(s, substr string) bool {
			return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
		},
		"split": strings.Split,
	})
	parsed, err := t.ParseGlob(filepath.Join("templates", "*.html"))
	if err != nil {
		log.Printf("[ERROR] Fallo al cargar plantillas: %v", err)
		return
	}
	tmplsLock.Lock()
	tmpls = parsed
	tmplsLock.Unlock()
}

func init() {
	loadTemplates()
}

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
		Theme:        "nothing",
	}
}

func getSettings(r *http.Request) AppSettings {
	cookie, err := r.Cookie("settings")
	if err == nil {
		val, _ := url.QueryUnescape(cookie.Value)
		s := getDefaultSettings()
		if err := json.Unmarshal([]byte(val), &s); err == nil {
			return s
		}
	}
	return getDefaultSettings()
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
	wroteHeader bool
	noBody      bool // 204/304: sin cuerpo, no se comprime ni se cierra el writer
}

func (w *gzipResponseWriter) start() {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	h := w.Header()
	// http.ServeFile/ServeContent ya declararon un Content-Length del cuerpo SIN
	// comprimir: si no se borra, el cliente corta la respuesta antes de tiempo y
	// en los payloads incompresibles el server aborta el stream (PNG de 1986 B
	// entregado en 15 B).
	h.Del("Content-Length")
	h.Set("Content-Encoding", "gzip")
	h.Add("Vary", "Accept-Encoding")
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	if code == http.StatusNoContent || code == http.StatusNotModified {
		w.noBody = true
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(code)
		return
	}
	w.start()
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		// Sniffea los bytes SIN comprimir: Go no setea Content-Type cuando
		// hay Content-Encoding, y sin esto Firefox muestra el HTML como texto.
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", http.DetectContentType(b))
		}
		w.start()
	}
	return w.Writer.Write(b)
}

// Flush expone http.Flusher para que los streams (SSE de /api/mcp) no queden
// bufferizados: sin este metodo el type-assert del handler falla en silencio.
func (w *gzipResponseWriter) Flush() {
	if gz, ok := w.Writer.(*gzip.Writer); ok {
		_ = gz.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// gzipPool evita allocar ~1 MB de buffers por request solo para comprimir.
var gzipPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

// skipGzip: re-comprimir lo que ya viene comprimido no gana nada y rompia el
// Content-Length de http.ServeFile. El SSE ademas necesita streaming sin capas.
func skipGzip(r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/api/mcp/") || r.URL.Path == "/crest" {
		return true
	}
	switch strings.ToLower(filepath.Ext(r.URL.Path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".ico", ".svgz",
		".woff", ".woff2", ".mp4", ".webm", ".zip", ".gz", ".br":
		return true
	}
	return false
}

func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") || skipGzip(r) {
			next.ServeHTTP(w, r)
			return
		}
		gz := gzipPool.Get().(*gzip.Writer)
		gz.Reset(w)
		gzw := &gzipResponseWriter{Writer: gz, ResponseWriter: w}
		defer func() {
			if gzw.wroteHeader && !gzw.noBody {
				_ = gz.Close()
			}
			gzipPool.Put(gz)
		}()
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
				r.RemoteAddr = strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
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
		// CSP permisivo para CDNs conocidos pero bloqueando inline scripts maliciosos (parcialmente, requiere unsafe-inline para htmx y modales)
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://unpkg.com https://stats.cronix.com.ar; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: https://openweathermap.org https://upload.wikimedia.org https://cdn.register.f1.com https://a.espncdn.com https://static.promiedos.com.ar https://img.icons8.com; connect-src 'self' https://stats.cronix.com.ar;")
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
		err := sports.ForceUpdate(context.Background())
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

	// 3. Sismos y Alertas
	earthquake.ForceUpdate(context.Background())
	weather.ForceUpdateAlerts(context.Background())

	// Iniciar bucles de actualización en segundo plano
	sports.StartUpdateLoop()
	crypto.StartUpdateLoop()
	finance.StartUpdateLoop()
	earthquake.StartUpdateLoop()
	weather.StartAlertsLoop()

	// Si se pasa el argumento "mcp", ejecutar el servidor MCP y salir
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		log.Println("[MCP] Iniciando servidor MCP sobre stdio...")
		handleMCP()
		return
	}

	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", cacheMiddleware(fs)))

	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/test", handleTest)
	mux.HandleFunc("/weather", handleWeather)
	mux.HandleFunc("/crest", handleCrestProxy)
	mux.HandleFunc("/settings", handleSettings)
	mux.HandleFunc("/autocomplete/teams", rateLimit(handleAutocompleteTeams))
	mux.HandleFunc("/autocomplete/cities", rateLimit(handleAutocompleteCities))
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/f1-standings", handleF1Standings)
	mux.HandleFunc("/partidos-del-dia", handlePartidosDelDia)

	// API JSON v1
	mux.HandleFunc("/api/v1/weather", handleAPIWeather)
	mux.HandleFunc("/api/v1/sports", handleAPISports)
	mux.HandleFunc("/api/v1/finance", handleAPIFinance)
	mux.HandleFunc("/api/v1/earthquakes", handleAPIEarthquakes)
	mux.HandleFunc("/api/v1/holidays", handleAPIHolidays)
	mux.HandleFunc("/api/v1/all", handleAPIAll)

	// MCP vía SSE (Para comunicación entre contenedores)
	mux.HandleFunc("/api/mcp/sse", handleMCPSSE)
	mux.HandleFunc("/api/mcp/message", handleMCPMessage)

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
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
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
		City             string
		Team             string
	}{
		Holiday:          holidays.GetHolidayToday(now),
		UpcomingHolidays: holidays.GetUpcomingHolidays(now),
		ShowF1:           settings.ShowF1,
		ShowFootball:     settings.ShowFootball,
		ShowUFC:          settings.ShowUFC,
		ShowFinance:      settings.ShowFinance,
		Theme:            settings.Theme,
		City:             settings.City,
		Team:             settings.Team,
	}
	var buf bytes.Buffer
	tmplsLock.RLock()
	err := tmpls.ExecuteTemplate(&buf, "index.html", data)
	tmplsLock.RUnlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:   "hd_theme",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	w.Write(buf.Bytes())
}

func handleTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
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
		City             string
		Team             string
		CurrentTemp      float64
		WeatherAvailable bool
		WeatherCode      int
	}{
		Holiday:          holidays.GetHolidayToday(now),
		UpcomingHolidays: holidays.GetUpcomingHolidays(now),
		ShowF1:           settings.ShowF1,
		ShowFootball:     settings.ShowFootball,
		ShowUFC:          settings.ShowUFC,
		ShowFinance:      settings.ShowFinance,
		Theme:            "terminal", // FORZAR TEMA TERMINAL (entorno de test /test)
		City:             settings.City,
		Team:             settings.Team,
	}
	if wData, errW := weather.GetWeather(r.Context(), settings.Lat, settings.Lon); errW == nil {
		data.CurrentTemp = wData.Current.Temperature
		data.WeatherCode = wData.Current.WeatherCode
		data.WeatherAvailable = true
	}
	var buf bytes.Buffer
	tmplsLock.RLock()
	err := tmpls.ExecuteTemplate(&buf, "index.html", data)
	tmplsLock.RUnlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:  "hd_theme",
		Value: "terminal",
		Path:  "/",
	})
	w.Write(buf.Bytes())
}

type WeatherViewModel struct {
	Current       weather.CurrentWeather
	Forecast      []weather.ForecastItem
	Sunrise       string
	Sunset        string
	NextMatch     sports.UserSportsData
	TodayHoliday  *holidays.Holiday
	City          string
	Theme         string
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
	IsAvailable   bool
}

func handleWeather(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	settings := getSettings(r)

	weatherData, errW := weather.GetWeather(r.Context(), settings.Lat, settings.Lon)
	if errW != nil {
		log.Printf("Error obteniendo clima para %s: %v", settings.City, errW)
	}
	btcPrice := crypto.GetCachedBTC()
	ethPrice := crypto.GetCachedETH()
	trend := crypto.GetCachedBTCTrend()
	ethTrend := crypto.GetCachedETHTrend()

	rainProb := 0
	var current weather.CurrentWeather
	var forecast []weather.ForecastItem
	var sunrise, sunset string
	aqi := 0
	aqiDesc := "Desconocido"
	moonIcon := "moon"
	moonPhase := "---"
	isAvailable := false

	if weatherData != nil {
		isAvailable = true
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

	viewTheme := settings.Theme
	if c, errC := r.Cookie("hd_theme"); errC == nil && c.Value == "terminal" {
		viewTheme = "terminal"
	}

	viewModel := WeatherViewModel{
		Current:       current,
		Forecast:      forecast,
		Sunrise:       sunrise,
		Sunset:        sunset,
		NextMatch:     sports.GetSportsDataForUser(settings.Team),
		City:          settings.City,
		Theme:         viewTheme,
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
		Alert:         weather.GetSMNAlert(settings.Lat, settings.Lon),
		AQI:           aqi,
		AQIDesc:       aqiDesc,
		MoonIcon:      moonIcon,
		MoonPhaseName: moonPhase,
		IsAvailable:   isAvailable,
	}

	var buf bytes.Buffer
	tmplsLock.RLock()
	err := tmpls.ExecuteTemplate(&buf, "weather.html", viewModel)
	tmplsLock.RUnlock()
	if err != nil {
		log.Printf("Error renderizando weather: %v", err)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
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

		city := strings.TrimSpace(r.FormValue("city"))
		// Solo buscar coordenadas si la ciudad cambió para evitar fallos innecesarios
		if city != "" && city != s.City {
			if len(city) > 100 {
				http.Error(w, "Nombre de ciudad demasiado largo", http.StatusBadRequest)
				return
			}
			name, lat, lon, province, err := weather.SearchCity(city)
			if err != nil {
				log.Printf("[SETTINGS] Error buscando ciudad '%s': %v", city, err)
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
		val, err := json.Marshal(s)
		if err != nil {
			log.Printf("Error serializando settings: %v", err)
			http.Error(w, "Error interno", http.StatusInternalServerError)
			return
		}
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

		// Trigger para refrescar solo el clima inmediatamente sin recargar toda la página
		w.Header().Set("HX-Trigger", "refreshWeather")
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
	if strings.Contains(strings.ToLower(targetURL), ".svg") {
		ext = ".svg"
	}

	if name != "" {
		localPath := filepath.Join("static", "assets", "cache", name+ext)
		if info, err := os.Stat(localPath); err == nil && info.Size() > 500 {
			http.ServeFile(w, r, localPath)
			return
		}
	}

	resp, err := network.FetchSecure(targetURL)
	if err != nil {
		http.Error(w, "Error al obtener imagen", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	// S6. Validación de Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		http.Error(w, "URL no es una imagen válida", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=604800") // 7 días

	// Purgar selectivamente del cuerpo para guardar en disco
	var body bytes.Buffer
	tee := io.TeeReader(resp.Body, &body)
	_, _ = io.Copy(w, tee)

	if name != "" {
		localPath := filepath.Join("static", "assets", "cache", name+ext)
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			log.Printf("[CACHE] Error creando directorio para crest %s: %v", name, err)
		} else if err := os.WriteFile(localPath, body.Bytes(), 0644); err != nil {
			log.Printf("[CACHE] Error guardando crest %s: %v", name, err)
		}
	}
}

var (
	rateLimitMap = make(map[string]time.Time)
	rateLimitMu  sync.Mutex
)

func init() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			rateLimitMu.Lock()
			now := time.Now()
			for k, v := range rateLimitMap {
				if now.Sub(v) > 5*time.Minute {
					delete(rateLimitMap, k)
				}
			}
			rateLimitMu.Unlock()
		}
	}()
}

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

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func handleF1Standings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	standings, err := sports.FetchF1Standings(r.Context())
	if err != nil {
		log.Printf("Error fetching F1 standings: %v", err)
		http.Error(w, "Error obteniendo posiciones F1", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	tmplsLock.RLock()
	err = tmpls.ExecuteTemplate(&buf, "f1_standings.html", standings)
	tmplsLock.RUnlock()
	if err != nil {
		log.Printf("Error renderizando F1 standings: %v", err)
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

func handlePartidosDelDia(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	settings := getSettings(r)
	todayStr := time.Now().Format("02/01")

	var partidos []sports.MatchData
	for _, m := range sports.GetSportsData().AllMatches {
		if strings.Contains(m.Date, todayStr) {
			m.TeamCrest = sports.GetCrestURL(m.Team)
			m.OpponentCrest = sports.GetCrestURL(m.Opponent)
			partidos = append(partidos, m)
		}
	}

	viewData := struct {
		Partidos []sports.MatchData
		Team     string
	}{
		Partidos: partidos,
		Team:     settings.Team,
	}

	var buf bytes.Buffer
	tmplsLock.RLock()
	err := tmpls.ExecuteTemplate(&buf, "partidos.html", viewData)
	tmplsLock.RUnlock()
	if err != nil {
		log.Printf("[PARTIDOS] Error ejecutando template: %v", err)
		http.Error(w, "Error interno", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(buf.Bytes())
}
