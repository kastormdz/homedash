package holidays

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Los feriados salen de la API de argentinadatos (la misma fuente que ya usa el panel
// para las tasas y el riesgo pais). Antes eran una lista hardcodeada en el binario: un
// feriado nuevo por decreto no aparecia NUNCA, y los calculados a mano tenian dos bugs
// (ver calcularFeriados).
//
// El re-fetch es UNA VEZ POR MES: los puentes y los feriados por decreto se publican con
// meses de anticipacion, asi que un mes sobra. El cache vive en disco para que el mes se
// respete de verdad y no se reinicie con cada deploy del contenedor.
const (
	feriadosAPI    = "https://api.argentinadatos.com/v1/feriados/"
	cacheFeriados  = "/app/data/holidays_cache.json"
	cacheTTL       = 30 * 24 * time.Hour
	fetchTimeout   = 8 * time.Second
	maxNombreCorto = 30
)

// apiFeriado es un item de la API.
type apiFeriado struct {
	Fecha  string `json:"fecha"`
	Nombre string `json:"nombre"`
	Tipo   string `json:"tipo"` // inamovible | trasladable | puente
}

type feriadosCache struct {
	Year      int       `json:"year"`
	FetchedAt time.Time `json:"fetched_at"`
	Holidays  []Holiday `json:"holidays"`
}

var (
	feriadosMu    sync.RWMutex
	feriadosStats feriadosCache
	fetchMu       sync.Mutex // serializa los refrescos: N requests no disparan N fetches
)

// GetArgentinaHolidays devuelve los feriados del año en curso.
//
// Orden: cache vigente (memoria) -> cache vigente (disco) -> API -> cache vencido ->
// calculo local. La API manda; el calculo local es la red de seguridad para que el panel
// nunca se quede sin feriados si el tercero esta caido.
func GetArgentinaHolidays() []Holiday {
	year := time.Now().Year()

	feriadosMu.RLock()
	c := feriadosStats
	feriadosMu.RUnlock()

	if c.Year == year && len(c.Holidays) > 0 && time.Since(c.FetchedAt) < cacheTTL {
		return c.Holidays
	}

	if hs, ok := refrescar(year); ok {
		return hs
	}

	// La API no respondio. Un cache vencido del mismo año sigue siendo mejor que el
	// calculo local: tiene los puentes y los decretos.
	if c.Year == year && len(c.Holidays) > 0 {
		log.Printf("[HOLIDAYS] API sin respuesta; uso el cache del %s",
			c.FetchedAt.Format("2006-01-02"))
		return c.Holidays
	}

	log.Printf("[HOLIDAYS] API sin respuesta y sin cache: calculo local para %d", year)
	return calcularFeriados(year)
}

func refrescar(year int) ([]Holiday, bool) {
	fetchMu.Lock()
	defer fetchMu.Unlock()

	// otro request pudo refrescar mientras esperabamos el lock
	feriadosMu.RLock()
	c := feriadosStats
	feriadosMu.RUnlock()
	if c.Year == year && len(c.Holidays) > 0 && time.Since(c.FetchedAt) < cacheTTL {
		return c.Holidays, true
	}

	// 1. disco. Es lo que hace que el "una vez por mes" sobreviva a los reinicios.
	if d, ok := leerCacheDisco(); ok && d.Year == year && len(d.Holidays) > 0 &&
		time.Since(d.FetchedAt) < cacheTTL {
		feriadosMu.Lock()
		feriadosStats = d
		feriadosMu.Unlock()
		log.Printf("[HOLIDAYS] %d feriados de %d desde el cache (traidos el %s)",
			len(d.Holidays), year, d.FetchedAt.Format("2006-01-02"))
		return d.Holidays, true
	}

	// 2. API
	hs, err := traerDeAPI(year)
	if err != nil {
		log.Printf("[HOLIDAYS] no pude traer de la API: %v", err)
		return nil, false
	}

	nuevo := feriadosCache{Year: year, FetchedAt: time.Now(), Holidays: hs}
	feriadosMu.Lock()
	feriadosStats = nuevo
	feriadosMu.Unlock()
	guardarCacheDisco(nuevo)

	log.Printf("[HOLIDAYS] %d feriados de %d traidos de la API (proximo refresh en %d dias)",
		len(hs), year, int(cacheTTL.Hours()/24))
	return hs, true
}

func traerDeAPI(year int) ([]Holiday, error) {
	cliente := &http.Client{Timeout: fetchTimeout}
	resp, err := cliente.Get(fmt.Sprintf("%s%d", feriadosAPI, year))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var raw []apiFeriado
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("la API devolvio 0 feriados")
	}

	hs := make([]Holiday, 0, len(raw))
	for _, f := range raw {
		if f.Fecha == "" || f.Nombre == "" {
			continue
		}
		hs = append(hs, Holiday{Name: nombreCorto(f.Nombre), Date: f.Fecha})
	}
	if len(hs) == 0 {
		return nil, fmt.Errorf("la API devolvio %d items y ninguno usable", len(raw))
	}
	return hs, nil
}

// nombresCortos: la API manda el nombre legal completo y el badge de la barra es angosto
// (una linea, mono 10px). Se matchea por SUBSTRING y no por igualdad: el texto cambia
// entre años ("...General Martín Güemes (17/6)" vs "...General Martín Miguel de Güemes"),
// y con igualdad exacta el nombre se escapaba al truncado generico.
var nombresCortos = []struct{ contiene, corto string }{
	{"Güemes", "Gral. Güemes"},
	{"Belgrano", "Gral. Belgrano"},
	{"San Martín", "Gral. San Martín"},
	{"Memoria por la Verdad", "Día de la Memoria"},
	{"Guerra de Malvinas", "Día del Veterano"},
	{"Revolución de Mayo", "Revolución de Mayo"},
	{"Diversidad Cultural", "Diversidad Cultural"},
	{"Soberanía Nacional", "Soberanía Nacional"},
	{"Inmaculada Concepción", "Inmaculada Concepción"},
	{"Puente turístico", "Puente turístico"},
	{"Año nuevo", "Año Nuevo"},
}

func nombreCorto(n string) string {
	for _, m := range nombresCortos {
		if strings.Contains(n, m.contiene) {
			return m.corto
		}
	}
	r := []rune(n)
	if len(r) > maxNombreCorto {
		return string(r[:maxNombreCorto-1]) + "…"
	}
	return n
}

func leerCacheDisco() (feriadosCache, bool) {
	var c feriadosCache
	b, err := os.ReadFile(cacheFeriados)
	if err != nil {
		return c, false
	}
	if err := json.Unmarshal(b, &c); err != nil {
		log.Printf("[HOLIDAYS] cache en disco ilegible: %v", err)
		return c, false
	}
	return c, true
}

func guardarCacheDisco(c feriadosCache) {
	if err := os.MkdirAll(filepath.Dir(cacheFeriados), 0o755); err != nil {
		log.Printf("[HOLIDAYS] no pude crear %s: %v", filepath.Dir(cacheFeriados), err)
		return
	}
	b, err := json.Marshal(c)
	if err != nil {
		log.Printf("[HOLIDAYS] no pude serializar el cache: %v", err)
		return
	}
	if err := os.WriteFile(cacheFeriados, b, 0o644); err != nil {
		log.Printf("[HOLIDAYS] no pude guardar el cache: %v", err)
	}
}
