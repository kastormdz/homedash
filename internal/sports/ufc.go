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

	for i, comp := range mainEvent.Competitions {
		p1, p2 := "TBD", "TBD"
		if len(comp.Competitors) >= 2 {
			p1 = comp.Competitors[0].Athlete.DisplayName
			p2 = comp.Competitors[1].Athlete.DisplayName

			// Extraer fotos del evento principal (primera competición de la lista)
			if i == 0 {
				p1Headshot = comp.Competitors[0].Athlete.Headshot
				if p1Headshot == "" && comp.Competitors[0].ID != "" {
					p1Headshot = fmt.Sprintf("https://a.espncdn.com/i/headshots/mma/players/full/%s.png", comp.Competitors[0].ID)
				}
				p2Headshot = comp.Competitors[1].Athlete.Headshot
				if p2Headshot == "" && comp.Competitors[1].ID != "" {
					p2Headshot = fmt.Sprintf("https://a.espncdn.com/i/headshots/mma/players/full/%s.png", comp.Competitors[1].ID)
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
