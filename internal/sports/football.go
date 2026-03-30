package sports

import (
	"context"
	"encoding/json"
	"homedash/internal/common"
	"homedash/internal/network"
	"net/http"
	"strings"
	"time"
)

var TeamMapping = map[string]string{
	"boca": "boca", "river": "river", "racing": "racing", "independiente": "independiente",
	"sanlorenzo": "sanlorenzo", "talleres": "talleres", "instituto": "instituto", "godoycruz": "godoycruz",
	"velez": "velez", "rosariocentral": "rosariocentral", "newells": "newells", "huracan": "huracan",
	"defensayjusticia": "defensayjusticia", "estudiantes": "estudiantes", "lanus": "lanus", "banfield": "banfield",
	"barracas": "barracascentral", "belgrano": "belgrano", "union": "union", "tigre": "tigre", "platense": "platense",
	"atleticotucuman": "atleticotucuman", "argentinos": "argentinosjuniors", "centralcordoba": "centralcordobasde",
	"independienterivadavia": "independienterivadavia", "riestra": "riestra", "sarmiento": "sarmiento", "gimnasia": "gimnasialp",
	"racingdecordoba":        "afa",
	"estudiantesderiocuarto": "estudiantesrc",
	"estudiantesrc":          "estudiantesrc",
}

func GetTeams() []string {
	return []string{"Boca Juniors", "River Plate", "Racing Club", "Independiente", "San Lorenzo", "Talleres", "Godoy Cruz", "Vélez Sarsfield", "Rosario Central", "Newell's Old Boys", "Estudiantes", "Gimnasia", "Huracán", "Lanús"}
}

func GetCrestURL(teamName string) string {
	name := common.NormalizeName(teamName)

	if strings.Contains(name, "rivadavia") {
		return "/static/assets/crests/independienterivadavia.png"
	}
	if strings.Contains(name, "riocuarto") || strings.Contains(name, "estudiantesrc") {
		return "/static/assets/crests/estudiantesrc.png"
	}

	if name == "independiente" || (strings.Contains(name, "independiente") && strings.Contains(name, "avellaneda")) {
		return "/static/assets/crests/independiente.png"
	}

	if v, ok := TeamMapping[name]; ok {
		return "/static/assets/crests/" + v + ".png"
	}

	for k, v := range TeamMapping {
		if strings.Contains(name, k) {
			return "/static/assets/crests/" + v + ".png"
		}
	}

	return "/static/assets/crests/afa.png"
}

func guessBroadcaster(tournament string) string {
	t := strings.ToLower(tournament)
	if strings.Contains(t, "liga profesional") || strings.Contains(t, "primera división") || strings.Contains(t, "copa de la liga") {
		return "ESPN Premium / TNT Sports"
	}
	if strings.Contains(t, "copa argentina") {
		return "TyC Sports"
	}
	if strings.Contains(t, "libertadores") || strings.Contains(t, "sudamericana") {
		return "ESPN / Fox Sports"
	}
	if strings.Contains(t, "nacional") {
		return "TyC Sports / DSports"
	}
	return "A confirmar"
}

func fetchLiveMatches(ctx context.Context, url string) []MatchData {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := network.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	var sb ESPNScoreboard
	if err := json.NewDecoder(resp.Body).Decode(&sb); err != nil {
		return nil
	}
	var liveMatches []MatchData
	tournamentName := "Torneo Local"
	if len(sb.Leagues) > 0 {
		tournamentName = sb.Leagues[0].Name
	}
	for _, event := range sb.Events {
		if len(event.Competitions) == 0 {
			continue
		}
		comp := event.Competitions[0]
		var home, away, hScore, aScore string
		for _, c := range comp.Competitors {
			if c.HomeAway == "home" {
				home = c.Team.DisplayName
				hScore = c.Score
			} else {
				away = c.Team.DisplayName
				aScore = c.Score
			}
		}
		status := "SCHEDULED"
		if event.Status.Type.State == "in" {
			status = "LIVE"
		} else if event.Status.Type.State == "post" {
			status = "FINAL"
		}
		dateStr, timeStr := "", ""
		if t, err := parseToArgentina(event.Date); err == nil {
			dateStr = common.DaysAbbr[t.Weekday()] + " " + t.Format("02/01")
			timeStr = t.Format("15:04")
		}

		channel := ""
		for _, b := range comp.Broadcasts {
			if len(b.Names) > 0 {
				channel = b.Names[0]
				break
			}
		}

		if channel == "" || channel == "A confirmar" {
			for _, gb := range comp.GeoBroadcasts {
				if gb.Media.ShortName != "" {
					channel = gb.Media.ShortName
					break
				}
			}
		}

		if channel == "" || channel == "A confirmar" {
			for _, note := range comp.Notes {
				if strings.Contains(strings.ToLower(note.Text), "tv:") {
					parts := strings.Split(note.Text, ":")
					if len(parts) > 1 {
						channel = strings.TrimSpace(parts[1])
						break
					}
				}
			}
		}

		if channel == "" || channel == "A confirmar" {
			channel = guessBroadcaster(tournamentName)
		}

		channel = strings.ReplaceAll(channel, "ESP+", "ESPN Premium")
		channel = strings.ReplaceAll(channel, "TNTS", "TNT Sports")

		if home == "" || away == "" {
			continue
		}

		liveMatches = append(liveMatches, MatchData{
			Team: home, Opponent: away, Date: dateStr, Time: timeStr, Tournament: tournamentName,
			Stadium: comp.Venue.FullName, Channel: channel, HomeScore: hScore, AwayScore: aScore, Status: status, Clock: event.Status.DisplayClock,
		})
	}
	return liveMatches
}

func GetSportsDataForUser(teamName string) UserSportsData {
	all := GetSportsData()
	searchName := common.NormalizeName(teamName)
	if searchName == "" {
		searchName = "boca"
	}

	var bestMatch MatchData
	found := false
	lowestScore := 9999

	for _, f := range all.AllMatches {
		tName := common.NormalizeName(f.Team)
		oName := common.NormalizeName(f.Opponent)
		tourn := strings.ToLower(f.Tournament)

		matchTeam := false
		if searchName == "independiente" {
			if (tName == "independiente" || strings.Contains(tName, "avellaneda")) && !strings.Contains(tName, "rivadavia") {
				matchTeam = true
			}
			if (oName == "independiente" || strings.Contains(oName, "avellaneda")) && !strings.Contains(oName, "rivadavia") {
				matchTeam = true
			}
		} else if searchName == "racing" {
			if (tName == "racing" || strings.Contains(tName, "club")) && !strings.Contains(tName, "cordoba") {
				matchTeam = true
			}
			if (oName == "racing" || strings.Contains(oName, "club")) && !strings.Contains(oName, "cordoba") {
				matchTeam = true
			}
		} else if strings.Contains(tName, searchName) || strings.Contains(oName, searchName) {
			matchTeam = true
		}

		if matchTeam {
			statusScore := 1000
			if f.Status == "LIVE" {
				statusScore = 0
			} else if strings.Contains(f.Date, time.Now().Format("02/01")) {
				statusScore = 10
			}

			tournScore := 100
			if strings.Contains(tourn, "liga profesional") || strings.Contains(tourn, "primera division") {
				tournScore = 0
			} else if strings.Contains(tourn, "copa argentina") || strings.Contains(tourn, "libertadores") || strings.Contains(tourn, "sudamericana") {
				tournScore = 50
			}

			totalScore := statusScore + tournScore
			if totalScore < lowestScore {
				lowestScore = totalScore
				bestMatch = f
				found = true
			}
		}
	}

	if !found {
		bestMatch = MatchData{
			Tournament: "Sin partidos agendados", Team: teamName, Opponent: "N/A", Date: "--/--", Time: "--:--", Stadium: "A confirmar", Status: "SCHEDULED",
		}
	} else {
		bestMatch.TeamCrest = GetCrestURL(bestMatch.Team)
		bestMatch.OpponentCrest = GetCrestURL(bestMatch.Opponent)
	}

	return UserSportsData{Match: bestMatch, F1: all.F1, UFC: all.UFC}
}
