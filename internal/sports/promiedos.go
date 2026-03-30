package sports

import (
	"context"
	"encoding/json"
	"fmt"
	"homedash/internal/common"
	"homedash/internal/network"
	"io"
	"log"
	"net/http"
	"strings"
)

type PromiedosMatch struct {
	Home    string
	Away    string
	Channel string
	HScore  string
	AScore  string
	Status  string
	Clock   string
}

func fetchPromiedosChannels(ctx context.Context) []PromiedosMatch {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.promiedos.com.ar/", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := network.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[PROMIEDOS] Error de red: %v\n", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("[PROMIEDOS] Error HTTP: %d\n", resp.StatusCode)
		return nil
	}

	// B-4. Limitar lectura para prevenir OOM (Max 2MB)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil
	}
	html := string(body)

	startMarker := "<script id=\"__NEXT_DATA__\" type=\"application/json\">"
	startIdx := strings.Index(html, startMarker)
	if startIdx == -1 {
		log.Printf("[PROMIEDOS] No se encontró __NEXT_DATA__\n")
		return nil
	}
	startIdx += len(startMarker)
	endIdx := strings.Index(html[startIdx:], "</script>")
	if endIdx == -1 {
		return nil
	}

	jsonStr := html[startIdx : startIdx+endIdx]

	type NextData struct {
		Props struct {
			PageProps struct {
				Data struct {
					Leagues []struct {
						Games []struct {
							Teams []struct {
								Name string `json:"name"`
							} `json:"teams"`
							Scores     []interface{} `json:"scores"`
							TvNetworks []struct {
								Name string `json:"name"`
							} `json:"tv_networks"`
							Status struct {
								Name string `json:"name"`
								Enum int    `json:"enum"`
							} `json:"status"`
							GameTime int `json:"game_time"`
						} `json:"games"`
					} `json:"leagues"`
				} `json:"data"`
			} `json:"pageProps"`
		} `json:"props"`
	}

	var data NextData
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		log.Printf("[PROMIEDOS] Error unmarshal: %v\n", err)
		return nil
	}

	var matches []PromiedosMatch
	for _, league := range data.Props.PageProps.Data.Leagues {
		for _, game := range league.Games {
			if len(game.Teams) >= 2 {
				ch := ""
				if len(game.TvNetworks) > 0 {
					ch = game.TvNetworks[0].Name
				}

				hScore, aScore := "", ""
				if len(game.Scores) >= 2 {
					hScore = fmt.Sprintf("%v", game.Scores[0])
					aScore = fmt.Sprintf("%v", game.Scores[1])
				}

				status := "SCHEDULED"
				if game.Status.Enum == 2 {
					status = "LIVE"
				} else if game.Status.Enum == 3 {
					status = "FINAL"
				}

				pm := PromiedosMatch{
					Home:    common.NormalizeName(game.Teams[0].Name),
					Away:    common.NormalizeName(game.Teams[1].Name),
					Channel: ch,
					HScore:  hScore,
					AScore:  aScore,
					Status:  status,
					Clock:   fmt.Sprintf("%d'", game.GameTime),
				}
				matches = append(matches, pm)
			}
		}
	}
	log.Printf("[PROMIEDOS] Update finalizado. Partidos procesados: %d\n", len(matches))
	return matches
}
