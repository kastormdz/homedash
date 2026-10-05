package weather

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"homedash/internal/network"
)

// La AEMSA (Agencia Metropolitana de Mendoza) publica el pronóstico oficial de la
// provincia en https://aemsa.com.ar/api/v1/pronostico-general.
//
// Reemplaza al DACC (contingencias.mendoza.gov.ar/api/getPronostico.php) porque trae
// bastante más: el DACC da UN día en prosa; la AEM da TRES, y cada uno con la máxima y
// la mínima separadas por oasis (Norte / Centro / Sur, que es la división real de la
// provincia) más la cordillera. Los números del panel (temperatura actual, iconos) NO
// salen de acá: esos son de Open-Meteo, un modelo numérico. Esto es la lectura oficial
// del prognosticista, que va aparte.
const aemForecastURL = "https://aemsa.com.ar/api/v1/pronostico-general"

// aemCacheTTL: la AEM publica meta.ultima_modificacion; el pronóstico cambia pocas
// veces por día, así que una hora alcanza y evita pegarle al endpoint cada refresh.
const aemCacheTTL = time.Hour

// AEMOasis es una zona con su propia máxima y mínima.
type AEMOasis struct {
	Nombre string // "Norte", "Centro", "Sur"
	Maxima int
	Minima int
	Texto  string // "Parcialmente nublado con descenso de la temperatura..."
}

// AEMDay es un día de pronóstico oficial.
type AEMDay struct {
	Fecha         string     // "2026-10-05"
	DiaSemana     string     // "Lunes"
	FechaFormada  string     // "05-10-26"
	Situacion     string     // texto general, sin los oasis
	Oasis         []AEMOasis // en el orden que los manda la API
	Cordillera    string     // "Nevadas débiles"
	Maxima        int        // el del oasis Centro (el de Capital), o 0
	Minima        int
	TieneOasis    bool
	SitGeneralLng int
}

var (
	aemMutex  sync.RWMutex
	aemCache  []AEMDay
	aemCached time.Time
)

// GetAEMForecast devuelve los días de pronóstico oficial que haya (3 normalmente).
// Cache de 1 hora.
func GetAEMForecast(ctx context.Context) []AEMDay {
	aemMutex.RLock()
	if len(aemCache) > 0 && time.Since(aemCached) < aemCacheTTL {
		res := aemCache
		aemMutex.RUnlock()
		return res
	}
	aemMutex.RUnlock()

	days, err := fetchAEM(ctx)
	if err != nil || len(days) == 0 {
		// fallo silencioso con log: si no, un error de la AEM se ve en pantalla como
		// "no hay pronóstico", igual que si realmente no hubiera dato.
		return nil
	}
	aemMutex.Lock()
	aemCache = days
	aemCached = time.Now()
	aemMutex.Unlock()
	return days
}

// limpiaHTML saca las etiquetas y entidades del HTML embebido que manda la AEM en
// `situacion`: "Condiciones...<br>Oasis Norte: ...<br>Máxima: 21°C".
var reHTMLTag = regexp.MustCompile(`<[^>]*>`)

var reEntidad = strings.NewReplacer(
	"&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">",
	"&quot;", `"`, "&#39;", "'", "&aacute;", "á", "&eacute;", "é",
	"&iacute;", "í", "&oacute;", "ó", "&uacute;", "ú", "&ntilde;", "ñ",
)

func limpiarHTML(s string) string {
	s = reHTMLTag.ReplaceAllString(s, " ")
	return strings.TrimSpace(reEntidad.Replace(s))
}

// reOasis parte "Oasis Norte: Parcialmente nublado ... Máxima: 21°C | Mínima: 12°C".
// La descripción del oasis viene antes del "Máxima", así que se toma con un grupo
// no-greedy hasta "Máxima:".
var reOasis = regexp.MustCompile(
	`(?is)Oasis\s+(Norte|Centro|Sur)\s*:\s*(.*?)\s*Máxima\s*:\s*(-?\d+)\s*°?\s*C\s*\|?\s*M[ií]nima\s*:\s*(-?\d+)\s*°?`)

// reCordillera saca la línea final: "Cordillera: Nevadas débiles."
var reCordillera = regexp.MustCompile(`(?is)Cordillera\s*:\s*([^.|]+?)\.?$`)

// reSituacionGeneral toma lo que va antes del primer "Oasis".
func reSituacionGeneral(txt string) string {
	if i := strings.Index(strings.ToLower(txt), "oasis"); i > 0 {
		return strings.TrimSpace(strings.Trim(txt[:i], " .,"))
	}
	return strings.TrimSpace(strings.Trim(txt, " .,"))
}

func fetchAEM(ctx context.Context) ([]AEMDay, error) {
	resp, err := network.FetchSecureWithContext(ctx, aemForecastURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw struct {
		Data []struct {
			Fecha           string `json:"fecha"`
			DiaSemana       string `json:"dia_semana"`
			FechaFormateada string `json:"fecha_formateada"`
			Situacion       string `json:"situacion"`
		} `json:"data"`
		Meta struct {
			UltimaModificacion string `json:"ultima_modificacion"`
		} `json:"meta"`
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}
	if len(raw.Data) == 0 {
		return nil, nil
	}

	out := make([]AEMDay, 0, len(raw.Data))
	for _, it := range raw.Data {
		txt := limpiarHTML(it.Situacion)
		if txt == "" {
			continue
		}
		d := AEMDay{
			Fecha:        it.Fecha,
			DiaSemana:    it.DiaSemana,
			FechaFormada: it.FechaFormateada,
			Situacion:    reSituacionGeneral(txt),
		}

		for _, m := range reOasis.FindAllStringSubmatch(txt, -1) {
			o := AEMOasis{
				Nombre: m[1],
				Texto:  strings.TrimSpace(strings.Trim(m[2], " .,")),
			}
			o.Maxima, _ = strconv.Atoi(m[3])
			o.Minima, _ = strconv.Atoi(m[4])
			d.Oasis = append(d.Oasis, o)
			// Centro es el oasis de Capital; es el que va como dato principal.
			if o.Nombre == "Centro" {
				d.Maxima, d.Minima = o.Maxima, o.Minima
			}
		}
		d.TieneOasis = len(d.Oasis) > 0

		if m := reCordillera.FindStringSubmatch(txt); m != nil {
			d.Cordillera = strings.TrimSpace(m[1])
		}
		d.SitGeneralLng = len(d.Situacion)
		out = append(out, d)
	}
	return out, nil
}
