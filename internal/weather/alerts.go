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
	Link        string `xml:"link"`
}

type WeatherAlert struct {
	Text string
	Icon string
}

func GetSMNAlert(province string, city string) WeatherAlert {
	defaultAlert := WeatherAlert{Text: "Sin alertas actuales", Icon: "triangle-alert"}
	if province == "" {
		return defaultAlert
	}

	// 1. Prioridad: Mendoza (Contingencias Climáticas)
	pNorm := normalizeProvince(province)
	cNorm := normalizeProvince(city)
	if pNorm == "mendoza" || cNorm == "mendoza" || cNorm == "godoy cruz" {
		alert := getMendozaLocalAlert()
		if alert.Text != "Sin alertas actuales" {
			return alert
		}
	}

	// 2. Fallback: SMN
	alertMutex.RLock()
	if time.Since(alertCacheTime) < 30*time.Minute && alertCache != "" {
		cached := findAlertInCache(province)
		alertMutex.RUnlock()
		return cached
	}
	alertMutex.RUnlock()

	// Fetch new alerts from SMN
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
	items := rss.Channel.Items
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		alertCache += fmt.Sprintf("[%s] %s {%s}|", item.Title, item.Description, item.Link)
	}
	alertCacheTime = time.Now()
	alertMutex.Unlock()

	return findAlertInCache(province)
}

func getMendozaLocalAlert() WeatherAlert {
	// Scrapping simple de Contingencias Mendoza
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://www.contingencias.mendoza.gov.ar/web/pronostico.php")
	if err != nil {
		return WeatherAlert{Text: "Sin alertas actuales", Icon: "triangle-alert"}
	}
	defer resp.Body.Close()

	// Leemos el contenido (es pequeño)
	buf := make([]byte, 10240) // 10KB es suficiente
	n, _ := resp.Body.Read(buf)
	content := strings.ToLower(string(buf[:n]))

	// Buscamos patrones de alerta comunes en Mendoza
	if strings.Contains(content, "alerta de granizo") || strings.Contains(content, "tormentas fuertes") {
		return WeatherAlert{Text: "Alerta de Granizo (DACC)", Icon: "cloud-lightning"}
	}
	if strings.Contains(content, "viento zonda") || strings.Contains(content, "zonda en precordillera") {
		return WeatherAlert{Text: "Alerta Viento Zonda (DACC)", Icon: "wind"}
	}
	if strings.Contains(content, "heladas parciales") || strings.Contains(content, "heladas generales") {
		return WeatherAlert{Text: "Alerta de Heladas (DACC)", Icon: "thermometer-snowflake"}
	}

	return WeatherAlert{Text: "Sin alertas actuales", Icon: "triangle-alert"}
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
			link := ""
			
			if start := strings.Index(a, "["); start != -1 {
				if end := strings.Index(a, "]"); end != -1 {
					title = a[start+1:end]
					desc = a[end+1:]
				}
			}
			
			if start := strings.Index(desc, "{"); start != -1 {
				if end := strings.Index(desc, "}"); end != -1 {
					link = desc[start+1:end]
					desc = desc[:start]
				}
			}

			// Validar CADUCIDAD (Timestamp en link: CAP_20260323...)
			if link != "" {
				if idx := strings.Index(link, "CAP_"); idx != -1 && len(link) >= idx+16 {
					dateStr := link[idx+4 : idx+12] // YYYYMMDD
					alertTime, err := time.Parse("20060102", dateStr)
					if err == nil {
						// Si la alerta tiene más de 24 horas, la ignoramos
						if time.Since(alertTime) > 24*time.Hour {
							continue
						}
					}
					
					timeStr := link[idx+12 : idx+14] + ":" + link[idx+14 : idx+16]
					dayStr := link[idx+10 : idx+12]
					today := time.Now().Format("02")
					
					alert := summarizeAlert(title, desc)
					if dayStr == today {
						alert.Text = fmt.Sprintf("%s (%s)", alert.Text, timeStr)
					} else {
						// Aún si es de ayer, si pasó el filtro de 24h (ej: alerta nocturna), mostramos fecha
						alert.Text = fmt.Sprintf("%s (%s/%s)", alert.Text, dayStr, link[idx+8 : idx+10])
					}
					return alert
				}
			}
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
	if len(alert.Text) > 50 {
		alert.Text = alert.Text[:47] + "..."
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
