package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"homedash/internal/weather"
)

// El bloque del pronóstico oficial TIENE que renderizar.
//
// El bug: dentro de `{{with index .AEM 0}}` el contexto pasa a ser weather.AEMDay, asi
// que `{{len .AEM}}` explota con "can't evaluate field AEM in type weather.AEMDay" y la
// pagina entera devuelve HTTP 500. El DACC viejo no sufria esto porque no contaba nada
// del slice; la AEM trae 3 dias y necesita el `$` para volver al contexto raiz.
//
// Este test lo fija para las tres variantes: es exactamente el error que llevo a un
// deploy local antes de commitear, y un 500 en la home es lo peor que puede pasar.
func TestElPronosticoOficialRenderiza(t *testing.T) {
	if _, err := os.Stat("templates/test2_b.html"); err != nil {
		t.Skipf("correr desde la raiz del repo (faltan las plantillas): %v", err)
	}

	aem := []weather.AEMDay{{
		Fecha:        "2026-10-05",
		DiaSemana:    "Lunes",
		FechaFormada: "05-10-26",
		Situacion:    "Inestabilidad atmosférica con precipitaciones aisladas.",
		Cordillera:   "Nevadas débiles",
		TieneOasis:   true,
		Oasis: []weather.AEMOasis{
			{Nombre: "Norte", Maxima: 21, Minima: 12, Texto: "Parcialmente nublado"},
			{Nombre: "Centro", Maxima: 19, Minima: 7, Texto: "Mayormente nublado"},
			{Nombre: "Sur", Maxima: 21, Minima: 9, Texto: "Parcialmente nublado"},
		},
		Maxima: 19, Minima: 7,
	}}

	for _, v := range []string{"a", "b", "c"} {
		var buf bytes.Buffer
		// AEM es un campo promovido de WeatherViewModel: no entra en el literal del
		// struct (Go lo rechaza con promoted field in struct literal), se asigna después.
		vm := Test2ViewModel{Variante: v, Now: time.Now()}
		vm.AEM = aem
		if err := tmpls.ExecuteTemplate(&buf, "test2_"+v+".html", vm); err != nil {
			t.Fatalf("variante %s: no renderiza con AEM: %v", v, err)
		}
		html := buf.String()

		if !strings.Contains(html, "Oficial AEM") && !strings.Contains(html, "OFICIAL AEM") {
			t.Errorf("variante %s: no muestra el encabezado del pronóstico oficial", v)
		}
		if strings.Contains(html, "Oficial DACC") || strings.Contains(html, "OFICIAL DACC") {
			t.Errorf("variante %s: quedó una referencia al DACC viejo", v)
		}
		// la variante b muestra el bloque completo con el conteo de días y el tooltip
		if v == "b" {
			// html/template escapa el acento de "días": buscar sin la tilde.
			for _, want := range []string{"19°/7°", "Nevadas débiles", "Oasis Centro", "d"} {
				if !strings.Contains(html, want) {
					t.Errorf("variante b: falta %q en el bloque oficial", want)
				}
			}
		}
	}
}

// Sin AEM el bloque tiene que desaparecer limpio, no dejar un hueco ni un error.
func TestSinAEMElBloqueDesaparece(t *testing.T) {
	if _, err := os.Stat("templates/test2_b.html"); err != nil {
		t.Skipf("correr desde la raiz del repo (faltan las plantillas): %v", err)
	}
	for _, v := range []string{"a", "b", "c"} {
		var buf bytes.Buffer
		vm := Test2ViewModel{Variante: v, Now: time.Now()}
		if err := tmpls.ExecuteTemplate(&buf, "test2_"+v+".html", vm); err != nil {
			t.Fatalf("variante %s: no renderiza sin AEM: %v", v, err)
		}
		// El encabezado "Oficial AEM" es estatico en la variante c (esta siempre, haya
		// datos o no). Lo que tiene que desaparecer es el DATO: los numeros del oasis
		// y el texto de la situación.
		html := buf.String()
		for _, noDebe := range []string{"19°/7°", "Inestabilidad atmosférica", "Nevadas débiles"} {
			if strings.Contains(html, noDebe) {
				t.Errorf("variante %s: pintó %q sin tener datos de la AEM", v, noDebe)
			}
		}
	}
}
