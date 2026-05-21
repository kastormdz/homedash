package sports

import (
	"context"
	"encoding/json"
	"homedash/internal/common"
	"homedash/internal/network"
	"log"
	"net/http"
	"strings"
	"time"
)

func fetchWorldCupFixture(ctx context.Context) []WorldCupMatch {
	start := time.Now()
	// Rango de fechas para el Mundial 2026: 11 de junio al 19 de julio
	url := "https://site.api.espn.com/apis/site/v2/sports/soccer/fifa.world/scoreboard?lang=es&region=ar&limit=200&dates=20260611-20260719"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := network.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[WORLD CUP] Error fetch: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("[WORLD CUP] Status error: %d", resp.StatusCode)
		return nil
	}

	var sb ESPNScoreboard
	if err := json.NewDecoder(resp.Body).Decode(&sb); err != nil {
		log.Printf("[WORLD CUP] JSON error: %v", err)
		return nil
	}

	var matches []WorldCupMatch
	for _, event := range sb.Events {
		if len(event.Competitions) == 0 {
			continue
		}
		comp := event.Competitions[0]

		var home, away, hScore, aScore, hLogo, aLogo string
		for _, c := range comp.Competitors {
			if c.HomeAway == "home" {
				home = c.Team.DisplayName
				hScore = c.Score
				hLogo = c.Team.Logo
			} else {
				away = c.Team.DisplayName
				aScore = c.Score
				aLogo = c.Team.Logo
			}
		}

		status := "SCHEDULED"
		if event.Status.Type.State == "in" {
			status = "LIVE"
		} else if event.Status.Type.State == "post" {
			status = "FINAL"
		}

		dateStr, timeStr := "", ""
		var t time.Time
		if t, err = parseToArgentina(event.Date); err == nil {
			dateStr = common.DaysAbbr[t.Weekday()] + " " + t.Format("02/01")
			timeStr = t.Format("15:04")
		}

		stage := ""
		group := ""
		
		slug := strings.ToLower(event.Season.Slug)
		if slug == "" { slug = strings.ToLower(event.Name) }

		isGroupStage := strings.Contains(slug, "group-stage") || strings.Contains(slug, "fase de grupos")

		if isGroupStage {
			stage = "Fase de Grupos"
			group = "Fase de Grupos" // Lo usamos como flag
		} else {
			// Es una fase eliminatoria
			switch {
			case strings.Contains(slug, "round of 32") || strings.Contains(slug, "32avos"):
				stage = "32avos de Final"
			case strings.Contains(slug, "round of 16") || strings.Contains(slug, "octavos"):
				stage = "Octavos de Final"
			case strings.Contains(slug, "quarter") || strings.Contains(slug, "cuartos"):
				stage = "Cuartos de Final"
			case strings.Contains(slug, "semi") || strings.Contains(slug, "semi"):
				stage = "Semifinales"
			case strings.Contains(slug, "3rd") || strings.Contains(slug, "tercer"):
				stage = "Tercer Puesto"
			case strings.Contains(slug, "final"):
				stage = "Final"
			default:
				// Fallback si no machea nada, ej. "Round of 32 3 Winner vs..."
				stage = "Eliminatorias"
			}
		}

		venueCity := ""
		venueCountry := ""
		if len(comp.Venue.Address.City) > 0 {
			venueCity = comp.Venue.Address.City
		}
		if len(comp.Venue.Address.Country) > 0 {
			venueCountry = comp.Venue.Address.Country
		}
		
		// Fallback si no hay ciudad/país estructurado
		if venueCity == "" && len(comp.Venue.FullName) > 0 {
			venueCity = comp.Venue.FullName
		}

		matches = append(matches, WorldCupMatch{
			ID:           event.ID,
			Date:         dateStr,
			Time:         timeStr,
			Home:         home,
			Away:         away,
			HomeScore:    hScore,
			AwayScore:    aScore,
			HomeLogo:     hLogo,
			AwayLogo:     aLogo,
			Status:       status,
			Stage:        stage,
			Group:        group,
			VenueCity:    venueCity,
			VenueCountry: venueCountry,
			Tournament:   "Copa Mundial 2026",
		})
	}

	log.Printf("[WORLD CUP] Fixture cargado: %d partidos en %v", len(matches), time.Since(start))
	return matches
}
