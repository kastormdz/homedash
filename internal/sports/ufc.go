package sports

import (
	"context"
	"encoding/json"
	"fmt"
	"homedash/internal/common"
	"homedash/internal/network"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// headshotSize es el lado en px que se le pide a ESPN. Los headshots se muestran
// a 44-48 px (y 24 px en la lista): 96 cubre retina 2x con sobra.
const headshotSize = 96

// crestURLForHeadshot arma la URL del proxy /crest para un headshot, pidiéndole
// a ESPN que lo entregue YA reducido.
//
// ESPN sirve el original en 500x500 (~200 KB) y el panel lo muestra a 44 px; sus
// 10 headshots eran el 88% del peso de la página (2,2 MB). Su propio CDN lo
// reescala si se pide por /combiner con w/h: medido, 204.673 B -> 12.313 B
// (17x menos) en un PNG 96x96 válido, sin gastar CPU acá. El nombre de la caché
// lleva el tamaño para no seguir sirviendo un archivo de 500x500 ya guardado.
func crestURLForHeadshot(raw, id string) string {
	if raw == "" {
		return ""
	}
	if i := strings.Index(raw, "/i/headshots/"); i >= 0 {
		img := raw[i:] // el combiner quiere el path, sin el host
		raw = fmt.Sprintf("https://a.espncdn.com/combiner/i?img=%s&w=%d&h=%d", img, headshotSize, headshotSize)
		return fmt.Sprintf("/crest?url=%s&name=athlete_%s_%d", url.QueryEscape(raw), id, headshotSize)
	}
	// Sin patrón reconocido: se proxea tal cual, pero escapado (antes se
	// concatenaba crudo y un & en la URL rompía el query).
	return fmt.Sprintf("/crest?url=%s&name=athlete_%s", url.QueryEscape(raw), id)
}

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

// searchFighterID busca el ID de un peleador por su nombre en el buscador de ESPN. Hace
// falta porque en la cartelera los peleadores que debutan vienen sin $ref y sin ID, asi
// que getFighterHeadshot no tiene a quien preguntarle y la foto queda vacia.
func searchFighterID(ctx context.Context, name string) string {
	if name == "" || name == "TBD" {
		return ""
	}
	u := "https://site.web.api.espn.com/apis/common/v3/search?query=" +
		url.QueryEscape(name) + "&limit=5&type=player"
	resp, err := network.FetchSecureWithContext(ctx, u)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var d struct {
		Items []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return ""
	}
	norm := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
	want := norm(name)
	// 1) nombre exacto (el buscador devuelve homonimos y otros deportes)
	for _, it := range d.Items {
		if it.ID != "" && norm(it.DisplayName) == want {
			return it.ID
		}
	}
	// 2) todas las palabras presentes: ESPN suele agregar el apodo o el segundo nombre
	for _, it := range d.Items {
		if it.ID == "" {
			continue
		}
		ok := true
		for _, w := range strings.Fields(want) {
			if !strings.Contains(norm(it.DisplayName), w) {
				ok = false
				break
			}
		}
		if ok {
			return it.ID
		}
	}
	return ""
}

// ufcScoreboardURL es la URL del scoreboard de UFC.
//
// OJO con el parametro `dates`: ESPN NO acepta un rango "AAAAMMDD-AAAAMMDD" en este
// endpoint. Con rango responde HTTP 500 {"code":1008,"detail":"script error"} y, como
// el error no se logueaba, el panel caia en "Sin eventos" sin dar ninguna pista (paso con
// UFC 332: el evento estaba en el feed, con sus 14 peleas, y no se veia nada).
// El scoreboard SIN `dates` ya devuelve lo que hay de vigente, que es lo que queremos:
// filtrar por ventana despues, en Go, donde es barato. Un dia suelto ("20261003") SI
// funciona; el rango es lo unico roto.
func ufcScoreboardURL() string {
	return "https://site.web.api.espn.com/apis/site/v2/sports/mma/ufc/scoreboard"
}

func fetchLiveUFC(ctx context.Context) UFCMatch {
	resp, err := network.FetchSecureWithContext(ctx, ufcScoreboardURL())
	if err != nil {
		log.Printf("[UFC] no pude pedir el scoreboard: %v", err)
		return UFCMatch{EventName: "Sin eventos"}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[UFC] el scoreboard respondio HTTP %d", resp.StatusCode)
		return UFCMatch{EventName: "Sin eventos"}
	}

	var sb ESPNScoreboard
	if err := json.NewDecoder(resp.Body).Decode(&sb); err != nil {
		log.Printf("[UFC] no pude decodificar el scoreboard: %v", err)
		return UFCMatch{EventName: "Sin eventos"}
	}
	if len(sb.Events) == 0 {
		log.Printf("[UFC] el scoreboard vino sin eventos (0)")
		return UFCMatch{EventName: "Sin eventos"}
	}

	var mainEvent ESPNEvent
	found := false
	for _, event := range sb.Events {
		// Solo eventos numerados (UFC NNN) y UFC Fight Night; se omiten Contender Series y otros.
		if event.Status.Type.State == "post" {
			continue
		}
		short := event.ShortName
		if strings.Contains(short, "Contender") || strings.Contains(short, "Noche") {
			continue
		}
		mainEvent = event
		found = true
		break
	}
	if !found {
		// Sin próximos eventos aptos: tomar el primero no post, o el primero a secas
		for _, event := range sb.Events {
			if event.Status.Type.State != "post" {
				mainEvent = event
				found = true
				break
			}
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
				if id1 == "" {
					id1 = searchFighterID(ctx, p1Name) // debuta: viene sin ID
				}
				rawP1 = getFighterHeadshot(ctx, id1)
			}
			rawP2 := comp.Competitors[1].Athlete.Headshot
			if rawP2 == "" {
				if id2 == "" {
					id2 = searchFighterID(ctx, p2Name)
				}
				rawP2 = getFighterHeadshot(ctx, id2)
			}
			fightP1Headshot = crestURLForHeadshot(rawP1, id1)
			fightP2Headshot = crestURLForHeadshot(rawP2, id2)

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
