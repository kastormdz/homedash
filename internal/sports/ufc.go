package sports

import (
	"encoding/json"
	"fmt"
	"homedash/internal/common"
	"homedash/internal/network"
	"time"
)

func fetchLiveUFC() UFCMatch {
	now := time.Now()
	startDate := now.AddDate(0, 0, -2).Format("20060102")
	endDate := now.AddDate(0, 0, 30).Format("20060102")
	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/mma/ufc/scoreboard?dates=%s-%s", startDate, endDate)

	resp, err := network.FetchSecure(url)
	if err != nil {
		return UFCMatch{EventName: "Sin eventos"}
	}
	defer resp.Body.Close()

	var sb ESPNScoreboard
	if err := json.NewDecoder(resp.Body).Decode(&sb); err != nil || len(sb.Events) == 0 {
		return UFCMatch{EventName: "Sin eventos"}
	}

	mainEvent := sb.Events[0]
	var mainCard []string
	var p1Headshot, p2Headshot string

	// En UFC, el Main Event suele ser el ÚLTIMO de la lista de competitions.
	// Vamos a recorrer en reversa para que el Main Event esté primero en nuestra lista.
	for i := len(mainEvent.Competitions) - 1; i >= 0; i-- {
		comp := mainEvent.Competitions[i]
		p1, p2 := "TBD", "TBD"
		if len(comp.Competitors) >= 2 {
			p1 = comp.Competitors[0].Athlete.DisplayName
			p2 = comp.Competitors[1].Athlete.DisplayName

			// Extraer fotos del evento principal (el último en la lista original, ahora el primero en nuestro loop)
			if i == len(mainEvent.Competitions)-1 {
				id1 := comp.Competitors[0].ID
				id2 := comp.Competitors[1].ID
				
				rawP1 := comp.Competitors[0].Athlete.Headshot
				if rawP1 == "" && id1 != "" {
					rawP1 = fmt.Sprintf("https://a.espncdn.com/i/headshots/mma/players/full/%s.png", id1)
				}
				rawP2 := comp.Competitors[1].Athlete.Headshot
				if rawP2 == "" && id2 != "" {
					rawP2 = fmt.Sprintf("https://a.espncdn.com/i/headshots/mma/players/full/%s.png", id2)
				}

				// Pasar por el proxy local para cachear imágenes
				if rawP1 != "" {
					p1Headshot = fmt.Sprintf("/crest?url=%s&name=athlete_%s", rawP1, id1)
				}
				if rawP2 != "" {
					p2Headshot = fmt.Sprintf("/crest?url=%s&name=athlete_%s", rawP2, id2)
				}
			}

			fightStatus := ""
			if comp.Status.Type.State == "in" {
				fightStatus = " (EN VIVO)"
			}
			if comp.Status.Type.State == "post" {
				fightStatus = " (FINAL)"
			}

			mainCard = append(mainCard, p1+" vs "+p2+fightStatus)
		}
	}

	dateStr, timeStr := "--/--", "--:--"
	if t, err := parseToArgentina(mainEvent.Date); err == nil {
		dateStr = common.DaysAbbr[t.Weekday()] + " " + t.Format("02/01")
		timeStr = t.Format("15:04")
	}

	status := "SCHEDULED"
	if mainEvent.Status.Type.State == "in" {
		status = "LIVE"
	} else if mainEvent.Status.Type.State == "post" {
		status = "FINAL"
	}

	name := mainEvent.Name
	if name == "" {
		name = mainEvent.ShortName
	}
	return UFCMatch{EventName: name, Date: dateStr, Time: timeStr, Status: status, MainCard: mainCard, P1Headshot: p1Headshot, P2Headshot: p2Headshot}
}
