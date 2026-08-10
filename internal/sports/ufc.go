package sports

import (
	"context"
	"encoding/json"
	"fmt"
	"homedash/internal/common"
	"homedash/internal/network"
	"time"
)

// getFighterHeadshot consulta el endpoint core de ESPN para obtener la URL real
// del headshot del peleador. ESPN dejó de incluir athlete.headshot en el
// scoreboard, y el id del competidor no siempre coincide con el id de la imagen.
func getFighterHeadshot(ctx context.Context, id string) string {
	if id == "" {
		return ""
	}
	apiURL := fmt.Sprintf("https://sports.core.api.espn.com/v2/sports/mma/leagues/ufc/athletes/%s", id)
	resp, err := network.FetchSecureWithContext(ctx, apiURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var ath struct {
		Headshot *struct {
			Href string `json:"href"`
		} `json:"headshot"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ath); err != nil {
		return ""
	}
	if ath.Headshot != nil && ath.Headshot.Href != "" {
		return ath.Headshot.Href
	}
	return ""
}

func fetchLiveUFC(ctx context.Context) UFCMatch {
	now := time.Now()
	startDate := now.AddDate(0, 0, -2).Format("20060102")
	endDate := now.AddDate(0, 0, 30).Format("20060102")
	url := fmt.Sprintf("https://site.web.api.espn.com/apis/site/v2/sports/mma/ufc/scoreboard?dates=%s-%s", startDate, endDate)

	resp, err := network.FetchSecureWithContext(ctx, url)
	if err != nil {
		return UFCMatch{EventName: "Sin eventos"}
	}
	defer resp.Body.Close()

	var sb ESPNScoreboard
	if err := json.NewDecoder(resp.Body).Decode(&sb); err != nil || len(sb.Events) == 0 {
		return UFCMatch{EventName: "Sin eventos"}
	}

	var mainEvent ESPNEvent
	found := false
	for _, event := range sb.Events {
		if event.Status.Type.State != "post" {
			mainEvent = event
			found = true
			break
		}
	}
	if !found {
		mainEvent = sb.Events[0]
	}

	var fights []UFCFight
	var p1Headshot, p2Headshot string

	// En UFC, el Main Event suele ser el ÚLTIMO de la lista de competitions.
	// Vamos a recorrer en reversa para que el Main Event esté primero en nuestra lista.
	for i := len(mainEvent.Competitions) - 1; i >= 0; i-- {
		comp := mainEvent.Competitions[i]
		p1Name, p2Name := "TBD", "TBD"
		winner := 0
		if len(comp.Competitors) >= 2 {
			p1Name = comp.Competitors[0].Athlete.DisplayName
			p2Name = comp.Competitors[1].Athlete.DisplayName

			if comp.Competitors[0].Winner {
				winner = 1
			} else if comp.Competitors[1].Winner {
				winner = 2
			}

			// Fotos del evento principal (el último en la lista original, ahora el primero en nuestro loop)
			fightStatus := ""
			if comp.Status.Type.State == "in" {
				fightStatus = "EN VIVO"
			}
			if comp.Status.Type.State == "post" {
				fightStatus = "FINAL"
			}

			id1 := comp.Competitors[0].ID
			id2 := comp.Competitors[1].ID
			fightP1Headshot, fightP2Headshot := "", ""

			rawP1 := comp.Competitors[0].Athlete.Headshot
			if rawP1 == "" {
				rawP1 = getFighterHeadshot(ctx, id1)
			}
			rawP2 := comp.Competitors[1].Athlete.Headshot
			if rawP2 == "" {
				rawP2 = getFighterHeadshot(ctx, id2)
			}
			if rawP1 != "" {
				fightP1Headshot = fmt.Sprintf("/crest?url=%s&name=athlete_%s", rawP1, id1)
			}
			if rawP2 != "" {
				fightP2Headshot = fmt.Sprintf("/crest?url=%s&name=athlete_%s", rawP2, id2)
			}

			// Fotos del evento principal
			if i == len(mainEvent.Competitions)-1 {
				p1Headshot = fightP1Headshot
				p2Headshot = fightP2Headshot
			}

			fights = append(fights, UFCFight{
				P1:         p1Name,
				P2:         p2Name,
				Winner:     winner,
				Status:     fightStatus,
				P1Headshot: fightP1Headshot,
				P2Headshot: fightP2Headshot,
			})
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

	venue := ""
	if len(mainEvent.Competitions) > 0 {
		v := mainEvent.Competitions[0].Venue
		if v.FullName != "" {
			venue = v.FullName
		} else if v.Address.City != "" {
			venue = v.Address.City
			if v.Address.Country != "" {
				venue += ", " + v.Address.Country
			}
		}
	}

	return UFCMatch{EventName: name, Date: dateStr, Time: timeStr, Venue: venue, Status: status, Fights: fights, P1Headshot: p1Headshot, P2Headshot: p2Headshot}
}
