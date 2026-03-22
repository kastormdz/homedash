package earthquake

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

type EarthquakeData struct {
	Magnitude string
	Location  string
	Time      string
	Date      string
	IsNew     bool
	FullTime  time.Time `json:"-"` // Para ordenar
}

func GetLatestEarthquakes() []EarthquakeData {
	var allQuakes []EarthquakeData
	
	// Fetch from USGS
	usgs := fetchUSGS()
	allQuakes = append(allQuakes, usgs...)
	
	// Fetch from INPRES
	inpres := fetchINPRES()
	allQuakes = append(allQuakes, inpres...)
	
	// Eliminar duplicados aproximados (por tiempo y magnitud)
	uniqueQuakes := deduplicate(allQuakes)
	
	// Ordenar por tiempo descendente
	sort.Slice(uniqueQuakes, func(i, j int) bool {
		return uniqueQuakes[i].FullTime.After(uniqueQuakes[j].FullTime)
	})
	
	if len(uniqueQuakes) > 10 {
		uniqueQuakes = uniqueQuakes[:10]
	}
	
	return uniqueQuakes
}

func fetchUSGS() []EarthquakeData {
	client := &http.Client{Timeout: 5 * time.Second}
	url := "https://earthquake.usgs.gov/fdsnws/event/1/query?format=geojson&limit=10&minlatitude=-55&maxlatitude=-21&minlongitude=-74&maxlongitude=-53"
	
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 { return nil }
	defer resp.Body.Close()

	var result struct {
		Features []struct {
			Properties struct {
				Mag   float64 `json:"mag"`
				Place string  `json:"place"`
				Time  int64   `json:"time"`
			} `json:"properties"`
		} `json:"features"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil { return nil }

	var quakes []EarthquakeData
	now := time.Now()
	locART, _ := time.LoadLocation("America/Argentina/Buenos_Aires")
	if locART == nil { locART = time.FixedZone("ART", -3*60*60) }

	for _, f := range result.Features {
		p := f.Properties
		locName := p.Place
		if idx := strings.Index(locName, " of "); idx != -1 {
			locName = strings.TrimSpace(locName[idx+4:])
		}
		
		t := time.Unix(p.Time/1000, 0).In(locART)
		quakes = append(quakes, EarthquakeData{
			Magnitude: fmt.Sprintf("%.1f", p.Mag),
			Location:  locName,
			Time:      t.Format("15:04"),
			Date:      t.Format("02/01"),
			IsNew:     now.Sub(t).Hours() < 24,
			FullTime:  t,
		})
	}
	return quakes
}

type INPRESList struct {
	Items []INPRESItem `xml:"item"`
}

type INPRESItem struct {
	ID    string `xml:"idSismo"`
	Fecha string `xml:"fecha"`
	Hora  string `xml:"hora"`
	Prof  string `xml:"prof"`
	Mg    string `xml:"mg"`
	Prov  string `xml:"prov"`
}

func fetchINPRES() []EarthquakeData {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://www.inpres.gob.ar/mapa/sismos.xml")
	if err != nil || resp.StatusCode != 200 { return nil }
	defer resp.Body.Close()

	var list INPRESList
	if err := xml.NewDecoder(resp.Body).Decode(&list); err != nil { return nil }

	var quakes []EarthquakeData
	now := time.Now()
	locART, _ := time.LoadLocation("America/Argentina/Buenos_Aires")
	if locART == nil { locART = time.FixedZone("ART", -3*60*60) }

	for _, item := range list.Items {
		// INPRES fecha es "DD/MM", año lo inferimos del ID (primeros 4 chars) o actual
		year := now.Year()
		if len(item.ID) >= 4 {
			fmt.Sscanf(item.ID[:4], "%d", &year)
		}
		
		dateParts := strings.Split(item.Fecha, "/")
		if len(dateParts) != 2 { continue }
		
		fullDateStr := fmt.Sprintf("%d-%s-%sT%s:00", year, dateParts[1], dateParts[0], item.Hora)
		t, err := time.ParseInLocation("2006-01-02T15:04:05", fullDateStr, locART)
		if err != nil { continue }

		quakes = append(quakes, EarthquakeData{
			Magnitude: item.Mg,
			Location:  strings.Title(strings.ToLower(item.Prov)),
			Time:      t.Format("15:04"),
			Date:      t.Format("02/01"),
			IsNew:     now.Sub(t).Hours() < 24,
			FullTime:  t,
		})
	}
	return quakes
}

func deduplicate(quakes []EarthquakeData) []EarthquakeData {
	if len(quakes) < 2 { return quakes }
	
	var result []EarthquakeData
	for _, q := range quakes {
		isDup := false
		for _, r := range result {
			// Si la diferencia de tiempo es < 2 min y la magnitud es parecida
			timeDiff := q.FullTime.Sub(r.FullTime)
			if timeDiff < 0 { timeDiff = -timeDiff }
			
			if timeDiff < 2*time.Minute && q.Magnitude == r.Magnitude {
				isDup = true
				break
			}
		}
		if !isDup {
			result = append(result, q)
		}
	}
	return result
}
