package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"homedash/internal/holidays"
)

// El confeti del feriado tiene que caer a lo ancho de TODA la barra de arriba (reloj +
// feriado + KPIs), no dentro de la pildora del feriado.
//
// El bug: el wrapper vivia DENTRO del .ferbadge, que es inline-flex con overflow:hidden.
// Las particulas caian dentro de la pildora y quedaban recortadas ahi: 20 papelitos
// cayendo en 60px de ancho y 20px de alto.
func TestElConfetiVaEnLaBarraYNoEnLaPildora(t *testing.T) {
	if _, err := os.Stat("templates/test2_b.html"); err != nil {
		t.Skipf("correr desde la raiz del repo: %v", err)
	}

	feriado := []holidays.Holiday{{Name: "Diversidad Cultural", Date: time.Now().Format("2006-01-02")}}

	for _, v := range []string{"a", "b", "c"} {
		var buf bytes.Buffer
		vm := Test2ViewModel{Variante: v, Now: time.Now(), Holiday: &feriado[0]}
		if err := tmpls.ExecuteTemplate(&buf, "test2_"+v+".html", vm); err != nil {
			t.Fatalf("variante %s: no renderiza con feriado: %v", v, err)
		}
		html := buf.String()

		if !strings.Contains(html, "confetti-wrapper barra") {
			t.Errorf("variante %s: falta el wrapper .barra (el confeti no cae en la barra)", v)
		}
		if !strings.Contains(html, "confetti") {
			t.Errorf("variante %s: no hay confeti con feriado", v)
		}

		// El wrapper NO puede estar dentro del ferbadge: se busca un ferbadge que contenga
		// un confetti-wrapper en su interior.
		for _, marca := range []string{`<span class="ferbadge"`, `<span class="ferbadge `} {
			for i := strings.Index(html, marca); i >= 0; i = strings.Index(html[i+1:], marca) {
				if i < 0 {
					break
				}
				fin := strings.Index(html[i:], "</span>")
				if fin < 0 {
					continue
				}
				if strings.Contains(html[i:i+fin], "confetti-wrapper") {
					t.Errorf("variante %s: el confeti sigue adentro del ferbadge", v)
				}
				break
			}
		}
	}
}

// Sin feriado no debe aparecer ni el confeti ni el wrapper.
func TestSinFeriadoNoHayConfeti(t *testing.T) {
	if _, err := os.Stat("templates/test2_b.html"); err != nil {
		t.Skipf("correr desde la raiz del repo: %v", err)
	}
	for _, v := range []string{"a", "b", "c"} {
		var buf bytes.Buffer
		vm := Test2ViewModel{Variante: v, Now: time.Now()}
		if err := tmpls.ExecuteTemplate(&buf, "test2_"+v+".html", vm); err != nil {
			t.Fatalf("variante %s: no renderiza sin feriado: %v", v, err)
		}
		if strings.Contains(buf.String(), "confetti-wrapper") {
			t.Errorf("variante %s: pintó confeti sin feriado", v)
		}
	}
}
