package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"homedash/internal/holidays"
)

// TestFeriadoEnLaBarra verifica el render del feriado en las tres variantes de /test2.
//
// Hoy no es feriado, asi que el dato se fuerza: si esto pasa, el dia que sea feriado la
// barra se ilumina, cae el confeti y aparece el nombre, sin tocar nada más.
// Tambien cubre el caso normal (tiene que mostrar el PROXIMO feriado) y que el glow
// no aparezca cuando no corresponde.
func TestFeriadoEnLaBarra(t *testing.T) {
	// OJO: main.go tiene un init() que llama a loadTemplates(), y ese ParseGlob es
	// relativo al CWD. Con `go test ./cmd/` el CWD es cmd/ y el proceso muere en el
	// init antes de llegar aca. Correr compilando el binario desde la raiz:
	//   go test -c -o /tmp/feriado.test ./cmd/ && cd <raiz> && /tmp/feriado.test -test.v
	if _, err := os.Stat("templates/test2_b.html"); err != nil {
		t.Skipf("correr desde la raiz del repo (faltan las plantillas): %v", err)
	}

	feriadoHoy := &holidays.Holiday{Name: "Día del Respeto a la Diversidad Cultural", Date: "2026-10-12"}
	upcoming := holidays.GetUpcomingHolidays(time.Now())

	for _, v := range []string{"a", "b", "c"} {
		// ---- caso 1: HOY es feriado ----
		var buf bytes.Buffer
		vm := Test2ViewModel{Variante: v, Now: time.Now(), Holiday: feriadoHoy, Upcoming: upcoming}
		if err := tmpls.ExecuteTemplate(&buf, "test2_"+v+".html", vm); err != nil {
			t.Fatalf("variante %s (feriado): no renderiza: %v", v, err)
		}
		html := buf.String()
		for _, want := range []string{"holiday-glow", "confetti-wrapper", "confetti", "Día del Respeto"} {
			if !strings.Contains(html, want) {
				t.Errorf("variante %s: falta %q en el HTML del feriado", v, want)
			}
		}
		if strings.Contains(html, "Próximo feriado") {
			t.Errorf("variante %s: muestra 'Próximo feriado' cuando HOY es feriado", v)
		}

		// ---- caso 2: no es feriado ----
		var buf2 bytes.Buffer
		vm2 := Test2ViewModel{Variante: v, Now: time.Now(), Upcoming: upcoming}
		if err := tmpls.ExecuteTemplate(&buf2, "test2_"+v+".html", vm2); err != nil {
			t.Fatalf("variante %s (normal): no renderiza: %v", v, err)
		}
		h2 := buf2.String()
		if strings.Contains(h2, "holiday-glow") {
			t.Errorf("variante %s: aplicó holiday-glow sin ser feriado", v)
		}
		if strings.Contains(h2, "confetti-wrapper") {
			t.Errorf("variante %s: tiró confeti sin ser feriado", v)
		}
		if !strings.Contains(h2, "Próximo feriado") {
			t.Errorf("variante %s: no muestra el próximo feriado", v)
		}
	}
}
