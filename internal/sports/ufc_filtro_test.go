package sports

import "testing"

// Los nombres reales que trae ESPN, medidos sobre los 52 eventos de 2026
// (site.api.espn.com/apis/site/v2/sports/mma/ufc/scoreboard?dates=2026).
// Hardcodeados a proposito: el filtro tiene que quedar fijo contra esto, no contra lo que
// yo recuerde.
var nombresUFC2026 = []struct {
	short    string
	name     string
	principal bool
}{
	// numerados (12)
	{"UFC 324", "UFC 324: Adesanya vs. Pereira", true},
	{"UFC 332", "UFC 332: Silva vs. Wang", true},
	{"UFC 335", "UFC 335: Topuria vs. Gaethje", true},
	// fight night (28, todos con el mismo shortName)
	{"UFC Fight Night", "UFC Fight Night: Preliminares", true},
	{"UFC Fight Night", "UFC Fight Night: Kape vs. Almubarak", true},
	// los que NO deben salir
	{"Dana White's Contender Series", "Dana White's Contender Series: Season 10, Week 9", false},
	{"Noche UFC", "Noche UFC: Silva vs. Delgado", false},
	{"UFC Freedom 250", "UFC Freedom 250: Topuria vs. Gaethje", false},
}

func TestEventEsPrincipal(t *testing.T) {
	for _, c := range nombresUFC2026 {
		e := ESPNEvent{ShortName: c.short, Name: c.name}
		if got := eventEsPrincipal(e); got != c.principal {
			t.Errorf("%q (name=%q): eventEsPrincipal = %v, quiero %v",
				c.short, c.name, got, c.principal)
		}
	}
}

// El caso que se colaba con el filtro viejo (lista negativa): UFC Freedom 250 no tiene
// "Contender" ni "Noche", asi que pasaba. Este es el bug.
func TestFreedom250QuedaAfuera(t *testing.T) {
	e := ESPNEvent{ShortName: "UFC Freedom 250", Name: "UFC Freedom 250: Topuria vs. Gaethje"}
	if eventEsPrincipal(e) {
		t.Fatal("UFC Freedom 250 no es un evento principal: tiene que quedar afuera")
	}
}

// Ojo: "UFC Freedom 250" TIENE un numero, asi que un regex ingenuo de "UFC + digitos"
// lo aceptaria. El filtro tiene que distinguir numero de evento (3 digitos, precedido
// por UFC y espacio) de un numero de edicion.
func TestElRegexNoConfundeEdicionConNumeroDeEvento(t *testing.T) {
	casos := []struct {
		texto string
		want  bool
	}{
		{"UFC 324", true},
		{"UFC 300: Almeida vs. Pereira", true},
		{"UFC Fight Night", false},
		{"UFC Freedom 250", false},
		{"Noche UFC: Silva vs. Delgado", false},
		{"Dana White's Contender Series", false},
	}
	for _, c := range casos {
		if got := reUFCEventoNumerado.MatchString(c.texto); got != c.want {
			t.Errorf("%q: reUFCEventoNumerado = %v, quiero %v", c.texto, got, c.want)
		}
	}
}
