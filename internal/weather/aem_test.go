package weather

import (
	"strconv"
	"strings"
	"testing"
)

// HTML real de https://aemsa.com.ar/api/v1/pronostico-general (2026-10-03). El parser
// tiene que sobrevivir a las etiquetas, a las entidades y al texto pegado: la API los
// manda sin espacios ("mañana.Oasis Norte:") y un split ingenuo por "Oasis" se rompe.
const aemHTMLReal = "<p><strong>Condiciones de inestabilidad atmosférica con probabilidad de " +
	"precipitaciones aisladas sobre sectores del Valle de Uco, sur  y este provincial durante " +
	"la madrugada y mañana.Oasis Norte: Parcialmente nublado con descenso de la temperatura, " +
	"vientos leves del sudeste con probabilidad de precipitaciones aisladas sobre  sectores " +
	"del este provincial.Máxima: 21°C | Mínima: 12°COasis Centro: Mayormente nublado con " +
	"descenso de la temperatura, vientos moderados del sudeste. probabilidad de " +
	"precipitaciones aisladas.Máxima: 19°C | Mínima: 7°COasis Sur: Parcialmente nublado con " +
	"descenso de la temperatura, vientos moderados del sudeste probabilidad de " +
	"precipitaciones aisladas sobre  sectores de San Rafael y General Alvear.Máxima: 21°C | " +
	"Mínima: 9°CCordillera: Nevadas débiles.</strong></p>"

func TestLimpiarHTML(t *testing.T) {
	got := limpiarHTML(aemHTMLReal)
	for _, mal := range []string{"<p>", "<strong>", "&nbsp;"} {
		if strings.Contains(got, mal) {
			t.Fatalf("quedó markup o entidad %q: %s", mal, got[:80])
		}
	}
	if !strings.Contains(got, "inestabilidad atmosférica") {
		t.Fatalf("se perdió el texto: %s", got[:80])
	}
}

func TestParseoDeOasis(t *testing.T) {
	txt := limpiarHTML(aemHTMLReal)
	ms := reOasis.FindAllStringSubmatch(txt, -1)
	if len(ms) != 3 {
		t.Fatalf("encontré %d oasis, quiero 3", len(ms))
	}

	esperado := map[string][2]int{"Norte": {21, 12}, "Centro": {19, 7}, "Sur": {21, 9}}
	for _, m := range ms {
		nombre, desc := m[1], strings.TrimSpace(m[2])
		mx, err := strconv.Atoi(m[3])
		if err != nil {
			t.Fatalf("máxima de %s no parsea: %q", nombre, m[3])
		}
		mn, err := strconv.Atoi(m[4])
		if err != nil {
			t.Fatalf("mínima de %s no parsea: %q", nombre, m[4])
		}
		e, ok := esperado[nombre]
		if !ok {
			t.Fatalf("oasis inesperado: %s", nombre)
		}
		if mx != e[0] || mn != e[1] {
			t.Fatalf("%s: %d/%d, esperaba %d/%d", nombre, mx, mn, e[0], e[1])
		}
		if desc == "" {
			t.Fatalf("%s quedó sin descripción", nombre)
		}
	}
}

func TestSituacionGeneralNoIncluyeLosOasis(t *testing.T) {
	txt := limpiarHTML(aemHTMLReal)
	sit := reSituacionGeneral(txt)
	if strings.Contains(sit, "Oasis") {
		t.Fatalf("la situación general se tragó los oasis: %s", sit[:90])
	}
	if !strings.Contains(sit, "inestabilidad") {
		t.Fatalf("la situación general salió vacía: %q", sit)
	}
	if strings.Contains(sit, "mañana.Oasis") {
		t.Fatalf("no cortó bien el límite: %q", sit)
	}
}

func TestCordillera(t *testing.T) {
	txt := limpiarHTML(aemHTMLReal)
	m := reCordillera.FindStringSubmatch(txt)
	if m == nil {
		t.Fatal("no extraje la cordillera")
	}
	if m[1] != "Nevadas débiles" {
		t.Fatalf("cordillera = %q, esperaba 'Nevadas débiles'", m[1])
	}
}

// El endpoint real, cuando hay red. Si no hay, skip: los tests de arriba ya fijan el parser.
func TestGetAEMForecastContraLaAPIReal(t *testing.T) {
	days := GetAEMForecast(t.Context())
	if len(days) == 0 {
		t.Skip("sin red o la API no devolvió datos")
	}
	t.Logf("AEM devolvió %d días", len(days))
	for _, d := range days {
		t.Logf("  %s %s | oasis=%d | general=%d chars | cordillera=%q | centro=%d/%d",
			d.FechaFormada, d.DiaSemana, len(d.Oasis), len(d.Situacion), d.Cordillera, d.Maxima, d.Minima)
		if d.Situacion == "" {
			t.Errorf("%s: situación general vacía", d.FechaFormada)
		}
		if !d.TieneOasis {
			t.Errorf("%s: no parseó ningún oasis", d.FechaFormada)
		}
		var centro bool
		for _, o := range d.Oasis {
			if o.Nombre == "Centro" {
				centro = true
			}
		}
		if !centro {
			t.Errorf("%s: falta el oasis Centro", d.FechaFormada)
		}
	}
}

// Un día sin cordillera no debe romper nada ni inventar una.
func TestSinCordilleraNoFalla(t *testing.T) {
	txt := limpiarHTML("<p>Oasis Centro: Nublado. Máxima: 18°C | Mínima: 9°C</p>")
	if m := reCordillera.FindStringSubmatch(txt); m != nil {
		t.Fatalf("inventó una cordillera: %q", m[1])
	}
	if reSituacionGeneral(txt) == "" {
		t.Fatal("la situación general no debe fallar si no hay Oasis")
	}
	// los oasis igual tienen que parsear
	if len(reOasis.FindAllStringSubmatch(txt, -1)) != 1 {
		t.Fatal("no parseó el único oasis")
	}
}
