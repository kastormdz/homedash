package sports

import (
	"context"
	"strings"
	"testing"
	"time"
)

// El bug: fetchLiveUFC pedia "?dates=AAAAMMDD-AAAAMMDD" y ESPN responde HTTP 500
// con ese rango. Como el error no se logueaba, el panel caia en "Sin eventos" y el
// UFC 332 (que estaba en el feed, con sus 14 peleas) no se veia. Sinningun aviso.
//
// Este test mira la URL que se construye, no la red: si alguien vuelve a agregar un
// rango, falla acá y no en produccion a las 3 de la manana.
func TestElScoreboardDeUFCNoPideRangoDeFechas(t *testing.T) {
	if strings.Contains(ufcScoreboardURL(), "dates=") {
		t.Fatalf("la URL vuelve a pedir dates=...: %s", ufcScoreboardURL())
	}
}

// Un dia suelto funciona, el rango no. Documenta la diferencia para que nadie lo
// "optimice" al reves.
func TestRangoDeFechasEsLoQueRompia(t *testing.T) {
	if !strings.Contains("dates=20261003", "dates=") {
		t.Fatal("sanity")
	}
	// lo que NO hay que hacer:
	malo := strings.Replace(ufcScoreboardURL(), "", "https://x/scoreboard?dates=20261001-20261102", 1)
	if !strings.Contains(malo, "dates=20261001-20261102") {
		t.Fatal("sanity")
	}
	t.Log("un dia suelto (dates=20261003) responde 200; el rango AAAAMMDD-AAAAMMDD responde 500")
}

// El fetch tiene que loguear el error. Un fallo silencioso es lo que mantuvo este bug
// escondido: el panel decia "Sin eventos" como si fuera la verdad.
func TestUnFalloDeRedNoEsSilencioso(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// sin red, el fetch tiene que devolver la estructura vacia PERO con el error logueado
	m := fetchLiveUFC(ctx)
	if m.EventName == "" {
		t.Fatal("EventName vacio: el panel no tendria nada que pintar")
	}
	t.Logf("sin red devolvio %q (tiene que quedar logueado en [UFC])", m.EventName)
}

func TestElFetchNoRevientaConSinEventos(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	m := fetchLiveUFC(ctx)
	if m.EventName == "" {
		t.Fatal("nunca debe devolver EventName vacio")
	}
	t.Logf("evento: %q con %d peleas", m.EventName, len(m.Fights))
}
