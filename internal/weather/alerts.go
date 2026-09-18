package weather

import (
	"context"
	"encoding/xml"
	"fmt"
	"homedash/internal/network"
	"io"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	alertMutex sync.RWMutex
	smnAlerts  []SMNAlert
	// link CAP -> contenido YA PARSEADO. Se guarda el contenido y no sólo el
	// timestamp para poder reconstruir la lista completa del feed sin volver a
	// bajar los XML.
	alertByLink = map[string]SMNAlert{}
	linkSeen    = map[string]time.Time{} // link CAP -> última vez que lo vimos
)

// alertRadiusKm es la tolerancia de distancia para dar una alerta por cercana.
// Los polígonos del SMN delimitan zonas geográficas (Cordillera, Llanura) y no
// ciudades: medido en producción, la alerta de Viento Zonda del Gran Mendoza
// pasa a 3,3 km de la Ciudad de Mendoza SIN contenerla. Con match estricto
// punto-en-polígono el dashboard nunca mostraba una alerta.
const alertRadiusKm = 20.0

type SMNAlert struct {
	Text     string
	Icon     string
	Severity string
	Polys    [][][2]float64 // uno o más polígonos {lat, lon}: una alerta cubre varias zonas
	At       time.Time
	Expires  time.Time // fin de vigencia (campo expires del CAP)
}

func StartAlertsLoop() {
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			ForceUpdateAlerts(ctx)
			cancel()
			time.Sleep(15 * time.Minute)
		}
	}()
}

func ForceUpdateAlerts(ctx context.Context) {
	// El SMN devuelve HTML intermitentemente en lugar del XML del feed
	// (incluso consistentemente desde algunas IPs). Reintentar varias veces
	// con backoff antes de abandonar.
	for attempt := 0; attempt < 5; attempt++ {
		if updateSMNAlerts(ctx) {
			return
		}
		select {
		case <-time.After(4 * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

// updateSMNAlerts baja el feed CAP del SMN y matchea las alertas por polígono.
// El RSS no nombra provincias: la zona real viene en cada XML CAP como un
// polígono de coordenadas, y el matcheo por nombre de zona es ambiguo
// ("Cordillera" cubre varias provincias). Se cachea cada XML por link para
// no refetchear alertas ya conocidas.
func updateSMNAlerts(ctx context.Context) bool {
	resp, err := network.FetchSecureWithContext(ctx, "https://ssl.smn.gob.ar/CAP/AR.php")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false
	}

	// El SMN a veces sirve el feed como HTML (misma data, distinto formato):
	// probar XML primero y caer a regex sobre el HTML si falla.
	items := parseCAPItems(body)
	if len(items) == 0 {
		log.Printf("[ALERTS] feed SMN sin items parseables (%d bytes)", len(body))
		return false
	}

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		actuales []SMNAlert
	)
	pruneCachedLinks()

	sem := make(chan struct{}, 10)
	for _, it := range items {
		it := it
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// El feed repite las mismas alertas en cada corrida: si ya tenemos
			// el contenido parseado se reusa tal cual (sin refetch).
			alertMutex.RLock()
			cached, ok := alertByLink[it.link]
			alertMutex.RUnlock()
			if ok {
				mu.Lock()
				actuales = append(actuales, cached)
				mu.Unlock()
				return
			}

			a, err := fetchCAPXML(ctx, it.title, it.link)
			if err != nil || len(a.Polys) == 0 {
				return
			}
			alertMutex.Lock()
			alertByLink[it.link] = a
			linkSeen[it.link] = time.Now()
			alertMutex.Unlock()
			mu.Lock()
			actuales = append(actuales, a)
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(actuales) == 0 {
		return false
	}
	// Publicar SIEMPRE la lista completa del feed actual. Antes se asignaban
	// sólo las alertas NUEVAS: cuando el feed traía una novedad, las otras
	// vigentes desaparecían (y cuando no traía ninguna, quedaban fantasmas).
	alertMutex.Lock()
	smnAlerts = actuales
	alertMutex.Unlock()

	exactos, cercanos := contarCobertura(-32.89, -68.82)
	log.Printf("[ALERTS] %d alertas SMN del feed (%d contienen Mendoza, %d a <=%.0f km)",
		len(actuales), exactos, cercanos, alertRadiusKm)
	return true
}

// contarCobertura cuenta cuántas alertas contienen el punto y cuántas quedan
// dentro del radio de tolerancia (para el log: es el diagnóstico que faltaba).
func contarCobertura(lat, lon float64) (exactos, cercanos int) {
	alertMutex.RLock()
	defer alertMutex.RUnlock()
	for _, a := range smnAlerts {
		cerca := false
		for _, poly := range a.Polys {
			if len(poly) < 3 {
				continue
			}
			if pointInPolygon(lat, lon, poly) {
				exactos++
				cerca = true
				break
			}
			if kmToPolygon(lat, lon, poly) <= alertRadiusKm {
				cerca = true
			}
		}
		if cerca {
			cercanos++
		}
	}
	return
}

type capItem struct {
	title string
	link  string
}

var capLinkRe = regexp.MustCompile(`https://ssl\.smn\.gob\.ar/feeds/CAP/xml_generados/CAP_\d+_[A-Za-z]+_[A-Za-z]+_alertas_alertas_\d+\.xml`)

// parseCAPItems extrae (title, link) del feed. Primero prueba decodificar XML
// RSS; si el server devolvió la variante HTML (sucede intermitentemente y por
// IP), extrae los links CAP con regex (la data es la misma).
func parseCAPItems(body []byte) []capItem {
	var rss struct {
		Channel struct {
			Items []struct {
				Title string `xml:"title"`
				Link  string `xml:"link"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(body, &rss); err == nil && len(rss.Channel.Items) > 0 {
		items := make([]capItem, 0, len(rss.Channel.Items))
		for _, it := range rss.Channel.Items {
			items = append(items, capItem{it.Title, it.Link})
		}
		return items
	}

	seen := map[string]bool{}
	var items []capItem
	for _, link := range capLinkRe.FindAllString(string(body), -1) {
		if seen[link] {
			continue
		}
		seen[link] = true
		items = append(items, capItem{capEvent(link), link})
	}
	return items
}

// capEvent extrae el fenómeno del filename: CAP_<ts>_<Event>_<Zona>_...
func capEvent(link string) string {
	base := strings.TrimSuffix(link, ".xml")
	parts := strings.Split(base, "_")
	for i, p := range parts {
		if len(p) == 14 && p[0] == '2' && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// pruneCachedLinks descarta links viejos (>48h) para que el mapa no crezca infinito.
func pruneCachedLinks() {
	alertMutex.Lock()
	defer alertMutex.Unlock()
	for link, t := range linkSeen {
		if time.Since(t) > 48*time.Hour {
			delete(linkSeen, link)
			delete(alertByLink, link)
		}
	}
}

func fetchCAPXML(ctx context.Context, title, link string) (SMNAlert, error) {
	var a SMNAlert
	resp, err := network.FetchSecureWithContext(ctx, link)
	if err != nil {
		return a, err
	}
	defer resp.Body.Close()

	// Area y polygon son SLICES: una alerta cubre varias zonas y cada zona
	// puede traer más de un polígono. Con structs simples se perdían todas
	// menos una, así que el matcheo fallaba para el resto de las zonas.
	var cap struct {
		Info []struct {
			Event    string `xml:"event"`
			Severity string `xml:"severity"`
			Desc     string `xml:"description"`
			Expires  string `xml:"expires"`
			Area     []struct {
				Polygon []string `xml:"polygon"`
			} `xml:"area"`
		} `xml:"info"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&cap); err != nil || len(cap.Info) == 0 {
		return a, fmt.Errorf("cap inválido")
	}

	info := cap.Info[0]
	event := info.Event
	if event == "" {
		event = title
	}
	sum := summarizeAlert(event)
	a.Text = sum.Text
	a.Icon = sum.Icon
	a.Severity = info.Severity
	for _, ar := range info.Area {
		for _, raw := range ar.Polygon {
			// Cada polígono queda SEPARADO: unirlos rompería el ray casting.
			if pts := parsePolygon(raw); len(pts) >= 3 {
				a.Polys = append(a.Polys, pts)
			}
		}
	}
	if t := capTimestamp(link); !t.IsZero() {
		a.At = t
	}
	if exp, err := time.Parse(time.RFC3339, strings.TrimSpace(info.Expires)); err == nil {
		a.Expires = exp
	}
	if len(a.Text) > 60 {
		a.Text = a.Text[:57] + "..."
	}
	return a, nil
}

// parsePolygon convierte "lat,lon lat,lon ..." de un polígono CAP en puntos.
func parsePolygon(s string) [][2]float64 {
	var pts [][2]float64
	for _, pair := range strings.Fields(s) {
		parts := strings.Split(pair, ",")
		if len(parts) != 2 {
			continue
		}
		lat, err1 := strconv.ParseFloat(parts[0], 64)
		lon, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil {
			pts = append(pts, [2]float64{lat, lon})
		}
	}
	return pts
}

// pointInPolygon: ray casting estándar. (lat, lon) dentro del polígono.
func pointInPolygon(lat, lon float64, poly [][2]float64) bool {
	inside := false
	j := len(poly) - 1
	for i := 0; i < len(poly); i++ {
		yi, xi := poly[i][0], poly[i][1]
		yj, xj := poly[j][0], poly[j][1]
		if (yi > lat) != (yj > lat) && lon < (xj-xi)*(lat-yi)/(yj-yi)+xi {
			inside = !inside
		}
		j = i
	}
	return inside
}

// capTimestamp extrae la fecha del id CAP_YYYYMMDDHHMMSS_...
func capTimestamp(link string) time.Time {
	i := strings.Index(link, "CAP_")
	if i == -1 || len(link) < i+18 {
		return time.Time{}
	}
	t, err := time.Parse("20060102150405", link[i+4:i+18])
	if err != nil {
		return time.Time{}
	}
	return t
}

func GetSMNAlert(latStr, lonStr string) WeatherAlert {
	defaultAlert := WeatherAlert{Text: "Sin alertas actuales", Icon: "triangle-alert"}
	lat, err1 := strconv.ParseFloat(latStr, 64)
	lon, err2 := strconv.ParseFloat(lonStr, 64)
	if err1 != nil || err2 != nil {
		return defaultAlert
	}

	now := time.Now()
	alertMutex.RLock()
	defer alertMutex.RUnlock()

	var mejor *SMNAlert
	mejorDist := math.MaxFloat64
	for i := range smnAlerts {
		a := &smnAlerts[i]
		if !a.Expires.IsZero() && now.After(a.Expires) {
			continue // vencida: el feed puede arrastrarlas un rato
		}
		for _, poly := range a.Polys {
			if len(poly) < 3 {
				continue
			}
			// Match exacto: el punto cae dentro del polígono.
			if pointInPolygon(lat, lon, poly) {
				return formatAlert(a, 0)
			}
			if d := kmToPolygon(lat, lon, poly); d < mejorDist {
				mejorDist, mejor = d, a
			}
		}
	}
	// Sin match exacto: los polígonos del SMN son zonas geográficas amplias y
	// suelen dejar afuera el centro urbano. Dentro del radio de tolerancia la
	// alerta aplica igual (se indica la distancia para que se entienda).
	if mejor != nil && mejorDist <= alertRadiusKm {
		return formatAlert(mejor, mejorDist)
	}
	return defaultAlert
}

// formatAlert arma el texto que ve el usuario. Si distKm > 0 la alerta se dio
// por cercanía y no por contener el punto exacto.
func formatAlert(a *SMNAlert, distKm float64) WeatherAlert {
	text := a.Text
	if !a.At.IsZero() {
		text = fmt.Sprintf("%s (%s)", text, a.At.Format("15:04"))
	}
	if distKm > 0 {
		text = fmt.Sprintf("%s · a %.0f km", text, distKm)
	}
	return WeatherAlert{Text: text, Icon: a.Icon}
}

// kmBetween: distancia haversine en km.
func kmBetween(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp := p2 - p1
	dl := (lon2 - lon1) * math.Pi / 180
	h := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}

// kmToPolygon: distancia mínima del punto a alguno de los lados del polígono.
func kmToPolygon(lat, lon float64, poly [][2]float64) float64 {
	best := math.MaxFloat64
	for i := range poly {
		y1, x1 := poly[i][0], poly[i][1]
		y2, x2 := poly[(i+1)%len(poly)][0], poly[(i+1)%len(poly)][1]
		dy, dx := y2-y1, x2-x1
		var d float64
		if dx == 0 && dy == 0 {
			d = kmBetween(lat, lon, y1, x1)
		} else {
			t := ((lat-y1)*dy + (lon-x1)*dx) / (dy*dy + dx*dx)
			if t < 0 {
				t = 0
			} else if t > 1 {
				t = 1
			}
			d = kmBetween(lat, lon, y1+t*dy, x1+t*dx)
		}
		if d < best {
			best = d
		}
	}
	return best
}

type WeatherAlert struct {
	Text string
	Icon string
}

func summarizeAlert(title string) WeatherAlert {
	t := strings.ToLower(title)
	alert := WeatherAlert{Text: title, Icon: "triangle-alert"}

	if strings.Contains(t, "zonda") {
		alert.Icon = "wind"
	} else if strings.Contains(t, "viento") {
		alert.Icon = "wind"
	} else if strings.Contains(t, "tormenta") {
		alert.Icon = "cloud-lightning"
	} else if strings.Contains(t, "lluvia") {
		alert.Icon = "cloud-rain"
	} else if strings.Contains(t, "nieve") || strings.Contains(t, "nevada") {
		alert.Icon = "snowflake"
	} else if strings.Contains(t, "calor") {
		alert.Icon = "sun"
	} else if strings.Contains(t, "frio") {
		alert.Icon = "thermometer-snowflake"
	}
	return alert
}
