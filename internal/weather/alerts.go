package weather

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	alertCache      string
	alertCacheTime  time.Time
	alertMutex      sync.RWMutex
)

type RSS struct {
	Channel Channel `xml:"channel"`
}

type Channel struct {
	Items []Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
}

type WeatherAlert struct {
	Text string
	Icon string
}

func GetSMNAlert(province string) WeatherAlert {
	defaultAlert := WeatherAlert{Text: "Sin alertas actuales", Icon: "triangle-alert"}
	if province == "" {
		return defaultAlert
	}

	alertMutex.RLock()
	if time.Since(alertCacheTime) < 30*time.Minute && alertCache != "" {
		cached := findAlertInCache(province)
		alertMutex.RUnlock()
		return cached
	}
	alertMutex.RUnlock()

	// Fetch new alerts
	resp, err := http.Get("https://ssl.smn.gob.ar/CAP/AR.php")
	if err != nil {
		return WeatherAlert{Text: "Error SMN", Icon: "cloud-off"}
	}
	defer resp.Body.Close()

	var rss RSS
	if err := xml.NewDecoder(resp.Body).Decode(&rss); err != nil {
		return WeatherAlert{Text: "Error Alertas", Icon: "cloud-off"}
	}

	alertMutex.Lock()
	alertCache = ""
	for _, item := range rss.Channel.Items {
		alertCache += fmt.Sprintf("[%s] %s|", item.Title, item.Description)
	}
	alertCacheTime = time.Now()
	alertMutex.Unlock()

	return findAlertInCache(province)
}

func findAlertInCache(province string) WeatherAlert {
	if alertCache == "" {
		return WeatherAlert{Text: "Sin alertas actuales", Icon: "triangle-alert"}
	}

	pNorm := normalizeProvince(province)
	alerts := strings.Split(alertCache, "|")
	
	for _, a := range alerts {
		if a == "" { continue }
		if strings.Contains(normalizeProvince(a), pNorm) {
			title := ""
			desc := a
			if start := strings.Index(a, "["); start != -1 {
				if end := strings.Index(a, "]"); end != -1 {
					title = a[start+1:end]
					desc = a[end+1:]
				}
			}

			return summarizeAlert(title, desc)
		}
	}

	return WeatherAlert{Text: "Sin alertas actuales", Icon: "triangle-alert"}
}

func summarizeAlert(title, desc string) WeatherAlert {
	t := strings.ToLower(title)
	d := strings.ToLower(desc)
	
	alert := WeatherAlert{Text: title, Icon: "triangle-alert"}

	// Detectar fenómeno e icono
	if strings.Contains(t, "viento") {
		alert.Icon = "wind"
		// Intentar extraer km/h
		if idx := strings.Index(d, "km/h"); idx != -1 {
			start := idx - 1
			for start > 0 && ((d[start] >= '0' && d[start] <= '9') || d[start] == ' ' || d[start] == 'y' || d[start] == '-') {
				start--
			}
			speed := strings.TrimSpace(desc[start+1:idx+4])
			if len(speed) > 4 {
				alert.Text = "Viento " + speed
			}
		}
	} else if strings.Contains(t, "tormenta") {
		alert.Icon = "cloud-lightning"
	} else if strings.Contains(t, "lluvia") {
		alert.Icon = "cloud-rain"
	} else if strings.Contains(t, "nieve") {
		alert.Icon = "snowflake"
	} else if strings.Contains(t, "calor") {
		alert.Icon = "sun"
	} else if strings.Contains(t, "frio") {
		alert.Icon = "thermometer-snowflake"
	}

	// Limitar largo
	if len(alert.Text) > 40 {
		alert.Text = alert.Text[:37] + "..."
	}

	return alert
}

func normalizeProvince(p string) string {
	p = strings.ToLower(p)
	p = strings.ReplaceAll(p, "á", "a")
	p = strings.ReplaceAll(p, "é", "e")
	p = strings.ReplaceAll(p, "í", "i")
	p = strings.ReplaceAll(p, "ó", "o")
	p = strings.ReplaceAll(p, "ú", "u")
	p = strings.ReplaceAll(p, "provincia de ", "")
	p = strings.ReplaceAll(p, "ciudad autonoma de ", "")
	return strings.TrimSpace(p)
}
