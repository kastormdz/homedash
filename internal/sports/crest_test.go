package sports

import (
	"strings"
	"testing"
)

// El escudo generérico (AFA) no debe mostrarse cuando el partido trae el logo de ESPN.
// Bug: el fallback a /crest solo se aplicaba al partido destacado, así que el modal
// "Partidos del día" recorre la lista completa pintaba afa.png en los equipos de
// Primera Nacional (6 de 9 partidos en un día real).
func TestCrestWithFallbackUsaElLogoDeESPN(t *testing.T) {
	// equipo desconocido + logo de ESPN -> proxy /crest
	got := CrestWithFallback("Tristán Suárez", "tristan suarez",
		"https://a.espncdn.com/i/teamlogos/soccer/500/10163.png")
	if strings.Contains(got, "afa.png") {
		t.Fatalf("quedo el escudo generico: %s", got)
	}
	if !strings.HasPrefix(got, "/crest?url=") {
		t.Fatalf("no uso el proxy /crest: %s", got)
	}
	// la URL de ESPN debe ir escapada
	if !strings.Contains(got, "a.espncdn.com%2Fi%2Fteamlogos") {
		t.Fatalf("la URL no esta escapada (QueryEscape): %s", got)
	}
}

// Si el equipo SI esta en el TeamMapping, gana el escudo local aunque ESPN traiga logo.
func TestCrestWithFallbackRespetaElEscudoLocal(t *testing.T) {
	got := CrestWithFallback("Boca Juniors", "boca juniors",
		"https://a.espncdn.com/i/teamlogos/soccer/500/9.png")
	if strings.HasPrefix(got, "/crest?") {
		t.Fatalf("el escudo local deberia ganar: %s", got)
	}
	if !strings.Contains(got, "/static/assets/crests/") {
		t.Fatalf("no devolvio un escudo local: %s", got)
	}
}

// Sin logo de ESPN no hay de donde sacarlo: se queda el generico (y el template
// no lo pinta, antes que inventar).
func TestCrestWithFallbackSinLogoDevuelveElGenerico(t *testing.T) {
	got := CrestWithFallback("Club Desconocido", "club desconocido", "")
	if got != crestaGenerica {
		t.Fatalf("devolvio %q, esperaba el generico %q", got, crestaGenerica)
	}
}

// La lista completa de partidos de hoy no debe quedar con escudos genericos.
// Corre contra el feed real: si hoy no hay partidos, no falla (no hay nada que assert).
func TestLosPartidosDeHoyNoQuedanConEscudoGenerico(t *testing.T) {
	partidos := GetSportsData().AllMatches
	if len(partidos) == 0 {
		t.Skip("el feed esta vacio")
	}
	genericos := 0
	conLogo := 0
	for _, m := range partidos {
		if m.TeamCrest == crestaGenerica && m.HomeLogo != "" {
			conLogo++
		}
		if m.OpponentCrest == crestaGenerica && m.AwayLogo != "" {
			conLogo++
		}
		if strings.Contains(m.TeamCrest, crestaGenerica) && m.HomeLogo == "" {
			genericos++
		}
	}
	t.Logf("partidos con logo de ESPN disponible: %d | sin logo (generico inevitable): %d", conLogo, genericos)
	if conLogo > 0 {
		t.Fatalf("hay %d escudos con logo de ESPN disponible que quedaron en el generico", conLogo)
	}
}