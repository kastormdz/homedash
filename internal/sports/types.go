package sports

import (
	"fmt"
	"time"
)

type MatchData struct {
	Team                 string
	Opponent             string
	Date                 string
	Time                 string
	RawDate              time.Time // Para ordenamiento y scoring
	Tournament           string
	Stadium              string
	TeamCrest            string
	OpponentCrest        string
	HomeLogo             string
	AwayLogo             string
	Channel              string
	HomeScore            string
	AwayScore            string
	Status               string
	Clock                string
	NormalizedTeam       string
	NormalizedOpponent   string
	NormalizedTournament string
	EventID              string // ID único del evento (ESPN)
	WasRescheduled       bool
	RescheduledNote      string // ej: "Antes: Lun 12/05 20:00"
}

type F1Session struct {
	Name            string
	Date            string
	Time            string
	WeatherIcon     string
	Passed          bool
	WasRescheduled  bool
	RescheduledNote string
}

type F1Race struct {
	GrandPrix string
	Circuit   string
	Country   string
	FlagURL   string
	PosterURL string
	Sessions  []F1Session
}

type F1DriverStanding struct {
	Pos      string
	Driver   string
	Team     string
	Points   string
	Wins     string
}

type F1ConstructorStanding struct {
	Pos    string
	Team   string
	Points string
	Wins   string
}

type F1Standings struct {
	Drivers      []F1DriverStanding
	Constructors []F1ConstructorStanding
}

type UFCFight struct {
	P1          string
	P2          string
	Winner      int    // 1 para P1, 2 para P2, 0 si no hay ganador
	Status      string // "EN VIVO", "FINAL", etc.
	P1Headshot  string
	P2Headshot  string
}

type UFCMatch struct {
	EventName       string
	Date            string
	Time            string
	Venue           string
	Status          string
	Fights          []UFCFight
	P1Headshot      string
	P2Headshot      string
	WasRescheduled  bool
	RescheduledNote string
}

type WorldCupMatch struct {
	ID              string
	Date            string
	Time            string
	Home            string
	Away            string
	HomeScore       string
	AwayScore       string
	HomeLogo        string
	AwayLogo        string
	Status          string
	Stage           string
	Group           string
	VenueCity       string
	VenueCountry    string
	Tournament      string
	WasRescheduled  bool
	RescheduledNote string
}

type SportsData struct {
	AllMatches []MatchData
	F1         F1Race
	UFC        UFCMatch
	WorldCup   []WorldCupMatch
}

type UserSportsData struct {
	Match    MatchData
	F1       F1Race
	UFC      UFCMatch
	WorldCup []WorldCupMatch
}

type ESPNEvent struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"shortName"`
	Date      string `json:"date"`
	Season    struct {
		Year int    `json:"year"`
		Type int    `json:"type"`
		Slug string `json:"slug"`
	} `json:"season"`
	Status struct {
		Type struct {
			State string `json:"state"`
		} `json:"type"`
		DisplayClock string `json:"displayClock"`
	} `json:"status"`
	Competitions []struct {
		Venue struct {
			FullName string `json:"fullName"`
			Address  struct {
				City    string `json:"city"`
				Country string `json:"country"`
			} `json:"address"`
		} `json:"venue"`
		Notes []struct {
			Text string `json:"text"`
		} `json:"notes"`
		Status struct {
			Type struct {
				State string `json:"state"`
			} `json:"type"`
		} `json:"status"`
		Competitors []struct {
			ID       string `json:"id"`
			HomeAway string `json:"homeAway"`
			Score    string `json:"score"`
			Winner   bool   `json:"winner"`
			Team     struct {
				DisplayName string `json:"displayName"`
				Logo        string `json:"logo"`
			} `json:"team"`
			Athlete struct {
				DisplayName string `json:"displayName"`
				Headshot    string `json:"headshot"`
			} `json:"athlete"`
		} `json:"competitors"`
		Broadcasts []struct {
			Names []string `json:"names"`
		} `json:"broadcasts"`
		GeoBroadcasts []struct {
			Media struct {
				ShortName string `json:"shortName"`
			} `json:"media"`
		} `json:"geoBroadcasts"`
	} `json:"competitions"`
}

type ESPNScoreboard struct {
	Leagues []struct {
		Name string `json:"name"`
	} `json:"leagues"`
	Events []ESPNEvent `json:"events"`
}

type ergastResponse struct {
	MRData struct {
		RaceTable struct {
			Races []struct {
				RaceName string `json:"raceName"`
				Date     string `json:"date"`
				Time     string `json:"time"`
				Circuit  struct {
					CircuitId string `json:"circuitId"`
					Location  struct {
						Lat     string `json:"lat"`
						Long    string `json:"long"`
						Country string `json:"country"`
					} `json:"location"`
				} `json:"circuit"`
				FirstPractice    ergastSession `json:"FirstPractice"`
				SecondPractice   ergastSession `json:"SecondPractice"`
				ThirdPractice    ergastSession `json:"ThirdPractice"`
				Qualifying       ergastSession `json:"Qualifying"`
				Sprint           ergastSession `json:"Sprint"`
				SprintQualifying ergastSession `json:"SprintQualifying"`
			} `json:"Races"`
		} `json:"RaceTable"`
	} `json:"MRData"`
}

type ergastCircuitLocation struct {
	Lat     string `json:"lat"`
	Long    string `json:"long"`
	Country string `json:"country"`
}

type ergastCircuit struct {
	CircuitId string                `json:"circuitId"`
	Location  ergastCircuitLocation `json:"location"`
}

type ergastSession struct {
	Date string `json:"date"`
	Time string `json:"time"`
}

func parseToArgentina(utcStr string) (time.Time, error) {
	if utcStr == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}

	// Lista de formatos posibles que envían las APIs (ESPN, Ergast)
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04Z", // Común en ESPN UFC
	}

	var t time.Time
	var err error
	for _, f := range formats {
		t, err = time.Parse(f, utcStr)
		if err == nil {
			break
		}
	}

	if err != nil {
		return time.Time{}, err
	}

	loc, _ := time.LoadLocation("America/Argentina/Buenos_Aires")
	if loc == nil {
		return t, nil // Fallback a UTC si no se puede cargar la loc
	}
	return t.In(loc), nil
}
