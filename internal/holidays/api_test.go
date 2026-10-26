package holidays

import (
	"testing"
	"time"
)

// La regla del art. 6 de la ley 27.399: los trasladables que caen martes o miercoles van
// al lunes ANTERIOR; jueves o viernes, al lunes siguiente.
//
// El bug que esto fija: se usaba nextMonday para todo, y en 2026 el 17/06 (miercoles)
// terminaba el 22/06, el mismo dia que Belgrano. Dos feriados pisados y uno invisible.
func TestLunesSegunLey(t *testing.T) {
	casos := []struct {
		entrada, quiero string
		porque          string
	}{
		{"2026-06-17", "2026-06-15", "miercoles -> lunes anterior"},
		{"2026-08-17", "2026-08-17", "ya es lunes"},
		{"2026-10-12", "2026-10-12", "ya es lunes"},
		{"2027-06-17", "2027-06-21", "jueves -> lunes siguiente"},
		{"2027-08-17", "2027-08-16", "martes -> lunes anterior"},
	}
	for _, c := range casos {
		d, err := time.Parse("2006-01-02", c.entrada)
		if err != nil {
			t.Fatalf("%s no parsea: %v", c.entrada, err)
		}
		got := lunesSegunLey(d).Format("2006-01-02")
		if got != c.quiero {
			t.Errorf("%s (%s): dio %s, quiero %s", c.entrada, c.porque, got, c.quiero)
		}
	}
}

// Belgrano (20/06) es INAMOVIBLE: estaba en la lista de trasladables y se movia.
func TestBelgranoEsInamovible(t *testing.T) {
	hs := calcularFeriados(2026)
	var belgrano, guemes string
	for _, h := range hs {
		switch h.Name {
		case "Gral. Belgrano":
			belgrano = h.Date
		case "Gral. Güemes":
			guemes = h.Date
		}
	}
	if belgrano != "2026-06-20" {
		t.Errorf("Belgrano: %q, quiero 2026-06-20 (es inamovible)", belgrano)
	}
	if guemes != "2026-06-15" {
		t.Errorf("Güemes: %q, quiero 2026-06-15 (miercoles -> lunes anterior)", guemes)
	}
}

// Y lo mas importante: dos feriados no pueden caer el mismo dia.
func TestNoHayFeriadosPisados(t *testing.T) {
	for _, year := range []int{2025, 2026, 2027, 2028} {
		hs := calcularFeriados(year)
		vistos := map[string]string{}
		for _, h := range hs {
			if otro, ya := vistos[h.Date]; ya && otro != h.Name {
				t.Errorf("%d: %s y %s caen los dos el %s", year, otro, h.Name, h.Date)
			}
			vistos[h.Date] = h.Name
		}
	}
}

// Soberanía Nacional (20/11) es trasladable, no fijo.
func TestSoberaniaEsTrasladable(t *testing.T) {
	// 2026-11-20 es viernes -> lunes siguiente = 2026-11-23
	var soberania string
	for _, h := range calcularFeriados(2026) {
		if h.Name == "Soberanía Nacional" {
			soberania = h.Date
		}
	}
	if soberania != "2026-11-23" {
		t.Errorf("Soberanía: %q, quiero 2026-11-23 (viernes -> lunes siguiente)", soberania)
	}
}

// Los nombres reales de la API, medidos. Todos tienen que quedar cortos para el badge.
func TestNombreCorto(t *testing.T) {
	casos := []struct{ api, quiero string }{
		{"Paso a la Inmortalidad del General Martín Güemes (17/6)", "Gral. Güemes"},
		{"Paso a la Inmortalidad del General Manuel Belgrano", "Gral. Belgrano"},
		{"Paso a la Inmortalidad del Gral. José de San Martín", "Gral. San Martín"},
		{"Día Nacional de la Memoria por la Verdad y la Justicia", "Día de la Memoria"},
		{"Día del Veterano y de los Caídos en la Guerra de Malvinas", "Día del Veterano"},
		{"Día de la Revolución de Mayo", "Revolución de Mayo"},
		{"Día del Respeto a la Diversidad Cultural", "Diversidad Cultural"},
		{"Día de la Soberanía Nacional (20/11)", "Soberanía Nacional"},
		{"Día de la Inmaculada Concepción de María", "Inmaculada Concepción"},
		{"Puente turístico no laborable", "Puente turístico"},
		{"Año nuevo", "Año Nuevo"},
		{"Visita del papa León XIV", "Visita del papa León XIV"},
		{"Carnaval", "Carnaval"},
	}
	for _, c := range casos {
		if got := nombreCorto(c.api); got != c.quiero {
			t.Errorf("nombreCorto(%q) = %q, quiero %q", c.api, got, c.quiero)
		}
	}

	// ninguno puede pasarse del ancho del badge
	for _, c := range casos {
		if n := len([]rune(nombreCorto(c.api))); n > maxNombreCorto {
			t.Errorf("%q quedo en %d chars (max %d)", c.api, n, maxNombreCorto)
		}
	}
}

// Un nombre desconocido y larguisimo se recorta, no se deja pasar.
func TestNombreDesconocidoSeRecorta(t *testing.T) {
	largo := "Feriado Nacional Extraordinario de la República Argentina por Decreto"
	got := nombreCorto(largo)
	if n := len([]rune(got)); n > maxNombreCorto {
		t.Errorf("quedo en %d chars: %q", n, got)
	}
	if got == largo {
		t.Error("no recorto nada")
	}
}

// El cache tiene que ser mensual: si fuera diario, este test lo delata.
func TestCacheEsMensual(t *testing.T) {
	dias := int(cacheTTL.Hours() / 24)
	if dias != 30 {
		t.Errorf("cacheTTL = %d dias, quiero 30 (re-fetch una vez por mes)", dias)
	}
}
