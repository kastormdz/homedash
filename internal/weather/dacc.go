package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"homedash/internal/network"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// El DACC (Dirección de Contingencias Climáticas de Mendoza) publica el
// pronóstico oficial de la provincia en una API JSON que consume su propio
// sitio: https://contingencias.mendoza.gov.ar/pronostico.php hace
// fetch(`https://contingencias.mendoza.gov.ar/api/getPronostico.php?dia=YYYYMMDD`).
//
// El resto del DACC NO es consumible: la alerta de heladas sirve tablas PHP
// vacías (sólo encabezados) y el riesgo de heladas es un mapa con JS de
// Dreamweaver. Así que de ahí se aprovecha el pronóstico oficial, no alertas.
const daccForecastURL = "https://contingencias.mendoza.gov.ar/api/getPronostico.php?dia="

// flexInt tolera que el DACC mande el número como string ("27"), vacío o null:
// con int pelado, un solo campo así tiraba abajo el pronóstico completo.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	if err != nil {
		*f = 0 // un valor raro no debe cortar el dato
		return nil
	}
	*f = flexInt(n)
	return nil
}

// DACCForecast es el pronóstico oficial de un día.
type DACCForecast struct {
	Fecha      string // "18-09-2026"
	Pronostico string // texto oficial en prosa
	Maxima     int
	Minima     int
}

var (
	daccMutex  sync.RWMutex
	daccCache  []DACCForecast
	daccCached time.Time
)

// GetDACCForecast devuelve el pronóstico oficial de Mendoza para hoy y mañana.
// Cache de 1 hora: el DACC lo actualiza un par de veces por día.
func GetDACCForecast(ctx context.Context) []DACCForecast {
	daccMutex.RLock()
	if len(daccCache) > 0 && time.Since(daccCached) < time.Hour {
		res := daccCache
		daccMutex.RUnlock()
		return res
	}
	daccMutex.RUnlock()

	var out []DACCForecast
	for i := 0; i < 2; i++ {
		dia := time.Now().AddDate(0, 0, i).Format("20060102")
		f, err := fetchDACCDay(ctx, dia)
		if err != nil {
			log.Printf("[DACC] sin pronóstico para %s: %v", dia, err)
			continue
		}
		out = append(out, f)
	}
	if len(out) > 0 {
		daccMutex.Lock()
		daccCache = out
		daccCached = time.Now()
		daccMutex.Unlock()
	}
	return out
}

func fetchDACCDay(ctx context.Context, dia string) (DACCForecast, error) {
	var f DACCForecast
	resp, err := network.FetchSecureWithContext(ctx, daccForecastURL+dia)
	if err != nil {
		return f, err
	}
	defer resp.Body.Close()

	// Devuelve un array de un elemento; si no hay dato para esa fecha,
	// {"mensaje": "No hay pronóstico para la fecha indicada"} (que no matchea
	// el array y deja len == 0).
	var raw []struct {
		Fecha      string  `json:"Fecha"`
		Pronostico string  `json:"Pronostico"`
		Maxima     flexInt `json:"maxima"`
		Minima     flexInt `json:"minima"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&raw); err != nil {
		return f, err
	}
	if len(raw) == 0 || raw[0].Pronostico == "" {
		return f, fmt.Errorf("sin datos")
	}
	r := raw[0]
	f = DACCForecast{Fecha: r.Fecha, Pronostico: r.Pronostico, Maxima: int(r.Maxima), Minima: int(r.Minima)}
	return f, nil
}
