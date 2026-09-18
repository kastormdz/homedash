// Package snow trae la altura de nieve de la cordillera mendocina.
//
// Fuente: Open-Meteo (la MISMA que ya usa el clima del panel, sin API key), sobre
// los puntos representativos de las tres cuencas que riegan Mendoza:
//
//	Río Mendoza  -> Las Cuevas    (-32.82, -69.83)
//	Río Tunuyán  -> Valle Hermoso (-33.05, -69.40)
//	Río Atuel    -> El Sosneado   (-34.85, -69.85)
//
// No existe feed oficial consumible de altura de nieve: el SMN devuelve 403/404
// en sus rutas de nieve, el SNIH no expone API pública y el DGI publica el
// informe de derrame en HTML sin datos estructurados. Open-Meteo entrega
// snow_depth (m) y snowfall_sum (cm) por punto, que es un dato verificable y de
// la misma familia que el resto del panel.
package snow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"sync"
	"time"

	"homedash/internal/network"
)

// Punto es una estación de medición en la cordillera.
type Punto struct {
	Nombre   string // cuenca: "Río Mendoza"
	Estacion string // "Las Cuevas"
	Lat      float64
	Lon      float64
}

var puntos = []Punto{
	{"Río Mendoza", "Las Cuevas", -32.82, -69.83},
	{"Río Tunuyán", "Valle Hermoso", -33.05, -69.40},
	{"Río Atuel", "El Sosneado", -34.85, -69.85},
}

// Cuenca es el estado nival de una cuenca.
type Cuenca struct {
	Nombre      string
	Estacion    string
	Elevacion   int     // m.s.n.m. que reporta el modelo
	NieveSemana float64 // cm acumulados en los últimos 7 días
	Profundidad float64 // m: máximo de los próximos 7 días
	TempMin     float64 // °C
	TempMax     float64 // °C
}

var (
	snowMutex  sync.RWMutex
	snowCache  []Cuenca
	snowCached time.Time
)

// GetCordillera devuelve el estado nival de las cuencas (cache 1 h: es un dato
// que cambia lento y no vale la pena pedirlo en cada render).
func GetCordillera(ctx context.Context) []Cuenca {
	snowMutex.RLock()
	if len(snowCache) > 0 && time.Since(snowCached) < time.Hour {
		out := snowCache
		snowMutex.RUnlock()
		return out
	}
	snowMutex.RUnlock()

	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		res []Cuenca
	)
	for _, p := range puntos {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := fetchPunto(ctx, p)
			if err != nil {
				log.Printf("[SNOW] %s (%s): %v", p.Nombre, p.Estacion, err)
				return
			}
			mu.Lock()
			res = append(res, c)
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(res) > 0 {
		// orden estable (= orden de `puntos`) para que la UI no baile entre renders
		ordenado := make([]Cuenca, 0, len(res))
		for _, p := range puntos {
			for _, c := range res {
				if c.Nombre == p.Nombre {
					ordenado = append(ordenado, c)
				}
			}
		}
		snowMutex.Lock()
		snowCache = ordenado
		snowCached = time.Now()
		snowMutex.Unlock()
		return ordenado
	}
	// Si falla todo, se devuelve lo último bueno que haya en caché (si hay).
	snowMutex.RLock()
	defer snowMutex.RUnlock()
	return snowCache
}

func fetchPunto(ctx context.Context, p Punto) (Cuenca, error) {
	var c Cuenca
	q := url.Values{}
	q.Set("latitude", fmt.Sprintf("%.2f", p.Lat))
	q.Set("longitude", fmt.Sprintf("%.2f", p.Lon))
	q.Set("daily", "snowfall_sum,temperature_2m_max,temperature_2m_min")
	q.Set("hourly", "snow_depth")
	q.Set("timezone", "America/Argentina/Mendoza")
	q.Set("forecast_days", "7")

	resp, err := network.FetchSecureWithContext(ctx,
		"https://api.open-meteo.com/v1/forecast?"+q.Encode())
	if err != nil {
		return c, err
	}
	defer resp.Body.Close()

	var raw struct {
		Elevation float64 `json:"elevation"`
		Daily     struct {
			Snowfall []float64 `json:"snowfall_sum"`
			TMax     []float64 `json:"temperature_2m_max"`
			TMin     []float64 `json:"temperature_2m_min"`
		} `json:"daily"`
		Hourly struct {
			SnowDepth []*float64 `json:"snow_depth"`
		} `json:"hourly"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 512<<10)).Decode(&raw); err != nil {
		return c, err
	}

	c = Cuenca{
		Nombre:    p.Nombre,
		Estacion:  p.Estacion,
		Elevacion: int(raw.Elevation),
	}
	for _, v := range raw.Daily.Snowfall {
		c.NieveSemana += v
	}
	// snow_depth viene en metros, con null cuando no hay dato
	for _, v := range raw.Hourly.SnowDepth {
		if v != nil && *v > c.Profundidad {
			c.Profundidad = *v
		}
	}
	// La serie diaria viene ordenada: [0] es hoy.
	if len(raw.Daily.TMax) > 0 {
		c.TempMax = raw.Daily.TMax[0]
	}
	if len(raw.Daily.TMin) > 0 {
		c.TempMin = raw.Daily.TMin[0]
	}
	return c, nil
}
