package main

import (
	"encoding/json"
	"homedash/internal/earthquake"
	"homedash/internal/finance"
	"homedash/internal/holidays"
	"homedash/internal/sports"
	"homedash/internal/weather"
	"net/http"
	"strconv"
)

// --- API JSON Handlers ---

func handleAPIWeather(w http.ResponseWriter, r *http.Request) {
	settings := getSettings(r)
	lat := r.URL.Query().Get("lat")
	lon := r.URL.Query().Get("lon")
	if lat == "" || lon == "" {
		lat, lon = settings.Lat, settings.Lon
	} else if !validLatLon(lat, lon) {
		http.Error(w, `{"error":"lat/lon inválidos"}`, http.StatusBadRequest)
		return
	}
	data, err := weather.GetWeather(r.Context(), lat, lon)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(data)
}

func handleAPISports(w http.ResponseWriter, r *http.Request) {
	data := sports.GetSportsData()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(data)
}

func handleAPIFinance(w http.ResponseWriter, r *http.Request) {
	data := finance.GetCachedFinance()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(data)
}

func handleAPIEarthquakes(w http.ResponseWriter, r *http.Request) {
	data := earthquake.GetLatestEarthquakes()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(data)
}

func handleAPIHolidays(w http.ResponseWriter, r *http.Request) {
	data := holidays.GetArgentinaHolidays()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(data)
}

func handleAPIAll(w http.ResponseWriter, r *http.Request) {
	settings := getSettings(r)
	wData, _ := weather.GetWeather(r.Context(), settings.Lat, settings.Lon)
	sData := sports.GetSportsData()
	fData := finance.GetCachedFinance()
	qData := earthquake.GetLatestEarthquakes()
	hData := holidays.GetArgentinaHolidays()

	combined := map[string]interface{}{
		"weather":     wData,
		"sports":      sData,
		"finance":     fData,
		"earthquakes": qData,
		"holidays":    hData,
		"settings":    settings,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(combined)
}

// validLatLon chequea que lat y lon sean numeros dentro de rango antes de meterlos en
// la query saliente a Open-Meteo: sin esto se les podia inyectar cualquier cosa.
func validLatLon(lat, lon string) bool {
	la, err1 := strconv.ParseFloat(lat, 64)
	lo, err2 := strconv.ParseFloat(lon, 64)
	return err1 == nil && err2 == nil && la >= -90 && la <= 90 && lo >= -180 && lo <= 180
}
