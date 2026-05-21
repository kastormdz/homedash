package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"homedash/internal/common"
	"homedash/internal/network"
	"math"
	"net/url"
	"sync"
	"time"
)

var (
	// Caché global compartida entre usuarios pero separada por ciudad
	weatherCache = make(map[string]*cachedWeatherItem)
	cityCache    = make(map[string]cityResult) // Nueva caché para ciudades
	weatherMutex sync.RWMutex
)

const maxCacheEntries = 100

type cityResult struct {
	Name      string
	Lat       string
	Lon       string
	Province  string
	Timestamp time.Time
}

type cachedWeatherItem struct {
	Data      *WeatherResponse
	Timestamp time.Time
}

const (
	BaseURL      = "https://api.open-meteo.com/v1/forecast"
	AQIURL       = "https://air-quality-api.open-meteo.com/v1/air-quality"
	GeocodingURL = "https://geocoding-api.open-meteo.com/v1/search"
)

type WeatherResponse struct {
	Current       CurrentWeather `json:"current"`
	Daily         DailyForecast  `json:"daily"`
	AQI           int            // US AQI
	AQIDesc       string         // Descripción (Bueno, Moderado, etc.)
	MoonIcon      string         // Icono de Lucide para la luna
	MoonPhaseName string         // Nombre de la fase
}

type CurrentWeather struct {
	Temperature  float64 `json:"temperature_2m"`
	ApparentTemp float64 `json:"apparent_temperature"`
	Humidity     int     `json:"relative_humidity_2m"`
	WindSpeed    float64 `json:"wind_speed_10m"`
	WeatherCode  int     `json:"weather_code"`
	UVIndex      float64 `json:"uv_index"`
}

type DailyForecast struct {
	Time           []string  `json:"time"`
	WeatherCode    []int     `json:"weather_code"`
	TemperatureMax []float64 `json:"temperature_2m_max"`
	TemperatureMin []float64 `json:"temperature_2m_min"`
	UVMax          []float64 `json:"uv_index_max"`
	RainProb       []int     `json:"precipitation_probability_max"`
	Sunrise        []string  `json:"sunrise"`
	Sunset         []string  `json:"sunset"`
}

type ForecastItem struct {
	Date string
	Code int
	Max  float64
	Min  float64
}

func GetWeather(ctx context.Context, lat, lon string) (*WeatherResponse, error) {
	if lat == "" {
		lat = "-32.89"
	}
	if lon == "" {
		lon = "-68.82"
	}
	key := lat + "," + lon

	weatherMutex.RLock()
	item, ok := weatherCache[key]
	weatherMutex.RUnlock()

	if ok && time.Since(item.Timestamp) < 5*time.Minute {
		return item.Data, nil
	}

	// 4. Fetch FUERA del write lock para no bloquear otros requests
	data, err := GetWeatherWithClient(ctx, lat, lon)
	if err != nil {
		// FALLBACK: Si falla la API pero tenemos datos viejos en caché, devolverlos
		if ok {
			// Actualizar el timestamp para no reintentar inmediatamente (enfriamiento de 2 min)
			weatherMutex.Lock()
			if item, stillExists := weatherCache[key]; stillExists {
				item.Timestamp = time.Now().Add(-3 * time.Minute) // 5 - 3 = 2 min para el próximo intento
			}
			weatherMutex.Unlock()
			fmt.Printf("[WEATHER] Fallback a datos viejos para %s,%s debido a error: %v (reintento en 2min)\n", lat, lon, err)
			return item.Data, nil
		}
		return nil, err
	}

	weatherMutex.Lock()
	defer weatherMutex.Unlock()

	// Double check: otra goroutine pudo haber cacheado mientras hacíamos fetch
	if item, ok := weatherCache[key]; ok && time.Since(item.Timestamp) < 5*time.Minute {
		return item.Data, nil
	}

	// P-A. Eviction LRU: eliminar la entrada más antigua en vez de vaciar todo
	if len(weatherCache) >= maxCacheEntries {
		evictOldestWeather()
	}
	weatherCache[key] = &cachedWeatherItem{Data: data, Timestamp: time.Now()}
	return data, nil
}

func GetWeatherWithClient(ctx context.Context, lat, lon string) (*WeatherResponse, error) {
	var wg sync.WaitGroup
	wg.Add(2)

	data := &WeatherResponse{}
	var errW error
	var aqi int
	var aqiDesc string

	go func() {
		defer wg.Done()
		fullURL := fmt.Sprintf("%s?latitude=%s&longitude=%s&current=temperature_2m,relative_humidity_2m,apparent_temperature,wind_speed_10m,weather_code,uv_index&daily=weather_code,temperature_2m_max,temperature_2m_min,uv_index_max,precipitation_probability_max,sunrise,sunset&timezone=auto&forecast_days=7", BaseURL, url.QueryEscape(lat), url.QueryEscape(lon))
		resp, err := network.FetchSecureWithContext(ctx, fullURL)
		if err != nil {
			errW = err
			return
		}
		defer resp.Body.Close()
		errW = json.NewDecoder(resp.Body).Decode(data)
	}()

	go func() {
		defer wg.Done()
		aqiURL := fmt.Sprintf("%s?latitude=%s&longitude=%s&current=us_aqi", AQIURL, url.QueryEscape(lat), url.QueryEscape(lon))
		respA, err := network.FetchSecureWithContext(ctx, aqiURL)
		if err != nil {
			return
		}
		defer respA.Body.Close()
		var aqiResult struct {
			Current struct {
				USAQI int `json:"us_aqi"`
			} `json:"current"`
		}
		if err = json.NewDecoder(respA.Body).Decode(&aqiResult); err == nil {
			aqi = aqiResult.Current.USAQI
			aqiDesc = getAQIDescription(aqi)
		}
	}()

	wg.Wait()

	if errW != nil || data == nil {
		if errW == nil {
			errW = fmt.Errorf("no data returned from weather API")
		}
		return nil, fmt.Errorf("error al pedir el clima: %w", errW)
	}

	data.AQI = aqi
	data.AQIDesc = aqiDesc
	data.MoonIcon, data.MoonPhaseName = getMoonPhaseInfo()

	return data, nil
}

func evictOldestWeather() {
	var oldestKey string
	var oldestTime time.Time
	for k, v := range weatherCache {
		if oldestKey == "" || v.Timestamp.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.Timestamp
		}
	}
	if oldestKey != "" {
		delete(weatherCache, oldestKey)
	}
}

func evictOldestCity() {
	var oldestKey string
	var oldestTime time.Time
	for k, v := range cityCache {
		if oldestKey == "" || v.Timestamp.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.Timestamp
		}
	}
	if oldestKey != "" {
		delete(cityCache, oldestKey)
	}
}

func getAQIDescription(aqi int) string {
	if aqi <= 50 {
		return "Bueno"
	}
	if aqi <= 100 {
		return "Moderado"
	}
	if aqi <= 150 {
		return "No saludable (Sensibles)"
	}
	if aqi <= 200 {
		return "No saludable"
	}
	if aqi <= 300 {
		return "Muy poco saludable"
	}
	return "Peligroso"
}

func getMoonPhaseInfo() (string, string) {
	now := time.Now()
	refDate := time.Date(2000, 1, 6, 18, 14, 0, 0, time.UTC)
	lunation := 29.530588853

	diff := now.Sub(refDate).Hours() / 24.0
	phase := math.Mod(diff, lunation) / lunation

	if phase < 0.06 || phase > 0.94 {
		return "moon-star", "Nueva"
	}
	if phase < 0.19 {
		return "moon", "Creciente"
	}
	if phase < 0.31 {
		return "moon", "C. Creciente"
	}
	if phase < 0.44 {
		return "moon", "Gibosa Cr."
	}
	if phase < 0.56 {
		return "circle", "Llena"
	}
	if phase < 0.69 {
		return "sun-moon", "Gibosa Meng."
	}
	if phase < 0.81 {
		return "moon", "C. Menguante"
	}
	return "moon", "Menguante"
}

func SearchCity(cityName string) (name, lat, lon, province string, err error) {
	weatherMutex.RLock()
	if res, ok := cityCache[cityName]; ok {
		weatherMutex.RUnlock()
		return res.Name, res.Lat, res.Lon, res.Province, nil
	}
	weatherMutex.RUnlock()

	query := url.Values{}
	query.Set("name", cityName)
	query.Set("count", "1")
	query.Set("language", "es")
	query.Set("format", "json")

	resp, err := network.FetchSecure(GeocodingURL + "?" + query.Encode())
	if err != nil {
		return "", "", "", "", err
	}
	defer resp.Body.Close()

	var result struct {
		Results []struct {
			Name      string  `json:"name"`
			Country   string  `json:"country"`
			Admin1    string  `json:"admin1"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", "", "", err
	}

	if len(result.Results) == 0 {
		return "", "", "", "", fmt.Errorf("ciudad no encontrada")
	}

	res := result.Results[0]
	name = fmt.Sprintf("%s, %s", res.Name, res.Country)
	lat = fmt.Sprintf("%.2f", res.Latitude)
	lon = fmt.Sprintf("%.2f", res.Longitude)
	province = res.Admin1

	// 4. Cache fuera del lock de lectura inicial
	weatherMutex.Lock()
	defer weatherMutex.Unlock()
	// Double-check después de adquirir el write lock
	if cachedRes, ok := cityCache[cityName]; ok {
		return cachedRes.Name, cachedRes.Lat, cachedRes.Lon, cachedRes.Province, nil
	}
	// P-B. Eviction LRU para city cache
	if len(cityCache) >= maxCacheEntries {
		evictOldestCity()
	}
	cityCache[cityName] = cityResult{name, lat, lon, province, time.Now()}

	return name, lat, lon, province, nil
}

func (w *WeatherResponse) GetForecastList() []ForecastItem {
	var list []ForecastItem
	// S5. Fix Slice bounds panic: usar el largo mínimo disponible
	count := len(w.Daily.Time)
	if len(w.Daily.WeatherCode) < count {
		count = len(w.Daily.WeatherCode)
	}
	if len(w.Daily.TemperatureMax) < count {
		count = len(w.Daily.TemperatureMax)
	}
	if len(w.Daily.TemperatureMin) < count {
		count = len(w.Daily.TemperatureMin)
	}

	for i := 0; i < count; i++ {
		date, _ := time.Parse("2006-01-02", w.Daily.Time[i])
		list = append(list, ForecastItem{
			Date: common.DaysAbbr[date.Weekday()],
			Code: w.Daily.WeatherCode[i],
			Max:  w.Daily.TemperatureMax[i],
			Min:  w.Daily.TemperatureMin[i],
		})
	}
	return list
}

func FormatTime(fullTime string) string {
	if len(fullTime) < 16 {
		return "--:--"
	}
	return fullTime[11:16]
}
