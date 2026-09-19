package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"homedash/internal/sports"
)

// TestUFCLiveRender verifica que cuando la cartelera tiene resultados (evento en curso o
// terminado) se rendericen el ganador resaltado y el chip de estado. Hoy el feed trae las
// peleas programadas (Winner=0, Status=""), asi que sin forzar el dato no habria forma de
// comprobar que el render del vivo funciona.
//
// Correr compilando el binario desde la raiz (main.go tiene un init que lee templates/):
//
//	go test -c -o /tmp/x.test ./cmd/ && cd <raiz> && /tmp/x.test -test.run TestUFCLive
func TestUFCLiveRender(t *testing.T) {
	if _, err := os.Stat("templates/test2_b.html"); err != nil {
		t.Skipf("correr desde la raiz del repo: %v", err)
	}

	// NextMatch es un campo promovido desde WeatherViewModel: no entra en el literal.
	vm := Test2ViewModel{Variante: "b", Now: time.Now()}
	vm.ShowUFC = true // sin esto el bloque ni se renderiza ({{if .ShowUFC}})
	vm.NextMatch = sports.UserSportsData{
		UFC: sports.UFCMatch{
			EventName: "UFC Fight Night: Test vs Test",
			Venue:     "Arena de prueba",
			Fights: []sports.UFCFight{
				{P1: "Ganador Uno", P2: "Perdedor Dos", Winner: 1, Status: "FINAL"},
				{P1: "Uno En Curso", P2: "Dos En Curso", Winner: 0, Status: "EN VIVO"},
				{P1: "Programado A", P2: "Programado B", Winner: 0, Status: ""},
			},
		},
	}

	var buf strings.Builder
	if err := tmpls.ExecuteTemplate(&buf, "test2_b.html", vm); err != nil {
		t.Fatalf("no renderiza con la cartelera: %v", err)
	}
	html := buf.String()

	for _, want := range []string{
		`class="win"`,   // ganador resaltado (P1 gano)
		`class="loser"`, // perdedor atenuado
		"FINAL",         // chip de la pelea terminada
		"EN VIVO",       // chip de la pelea en curso
		"UFC Fight Night: Test vs Test",
		"class=\"fights\"", // la cartelera, separada del titulo
	} {
		if !strings.Contains(html, want) {
			t.Errorf("falta %q en el HTML del UFC en vivo", want)
		}
	}

	// el ganador tiene que estar marcado en la pelea correcta, no en cualquiera
	// (la clase va ANTES del nombre: <b class="win">Ganador Uno</b>)
	if !strings.Contains(html, `class="win">Ganador Uno`) {
		t.Error("el ganador no esta resaltado en su propia fila")
	}
	if !strings.Contains(html, `class="loser">Perdedor Dos`) {
		t.Error("el perdedor no esta atenuado en su propia fila")
	}
	// la pelea en curso no debe marcar ganador
	if strings.Contains(html, `class="win">Uno En Curso`) || strings.Contains(html, `class="loser">Uno En Curso`) {
		t.Error("una pelea EN VIVO no debe resaltar ganador")
	}
}
