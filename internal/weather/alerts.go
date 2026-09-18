package weather

import (
	"context"
	"encoding/xml"
	"fmt"
	"homedash/internal/network"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	alertMutex  sync.RWMutex
	smnAlerts   []SMNAlert
	cachedLinks = map[string]time.Time{} // link CAP -> última vez que lo vimos
)

type SMNAlert struct {
	Text     string
	Icon     string
	Severity string
	Poly     [][2]float64 // {lat, lon}
	At       time.Time
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
		mu     sync.Mutex
		wg     sync.WaitGroup
		alerts []SMNAlert
	)
	// Sincronizar el mapa de links conocidos fuera de la gorutina principal
	pruneCachedLinks()

	sem := make(chan struct{}, 10)
	for _, it := range items {
		it := it
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			alertMutex.RLock()
			_, seen := cachedLinks[it.link]
			alertMutex.RUnlock()
			if seen {
				return
			}

			a, err := fetchCAPXML(ctx, it.title, it.link)
			if err != nil || len(a.Poly) < 3 {
				return
			}
			mu.Lock()
			alerts = append(alerts, a)
			alertMutex.Lock()
			cachedLinks[it.link] = time.Now()
			alertMutex.Unlock()
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(alerts) == 0 {
		return true
	}
	alertMutex.Lock()
	smnAlerts = alerts
	alertMutex.Unlock()
	match := 0
	for _, a := range alerts {
		if pointInPolygon(-32.89, -68.82, a.Poly) {
			match++
		}
	}
	log.Printf("[ALERTS] %d alertas SMN activas (%d cubren Mendoza)", len(alerts), match)
	return true
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
	for link, t := range cachedLinks {
		if time.Since(t) > 48*time.Hour {
			delete(cachedLinks, link)
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

	var cap struct {
		Info []struct {
			Event    string `xml:"event"`
			Severity string `xml:"severity"`
			Desc     string `xml:"description"`
			Area     struct {
				Polygon string `xml:"polygon"`
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
	a.Poly = parsePolygon(info.Area.Polygon)
	if t := capTimestamp(link); !t.IsZero() {
		a.At = t
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

	alertMutex.RLock()
	defer alertMutex.RUnlock()
	for _, a := range smnAlerts {
		if pointInPolygon(lat, lon, a.Poly) {
			text := a.Text
			if !a.At.IsZero() {
				text = fmt.Sprintf("%s (%s)", text, a.At.Format("15:04"))
			}
			return WeatherAlert{Text: text, Icon: a.Icon}
		}
	}
	return defaultAlert
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
