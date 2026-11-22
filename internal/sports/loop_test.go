package sports

import (
	"testing"
	"time"
)

// intervaloDelLoop decide cuánto duerme el loop antes del próximo update.
//
// El caso que importa: tras un reboot de Titan la red puede no estar lista cuando arranca
// Docker, el ForceUpdate inicial falla y, sin esto, el panel se queda con el cache del
// arranque anterior hasta 15 minutos sin avisar.
func TestIntervaloDelLoop(t *testing.T) {
	casos := []struct {
		nombre       string
		primero      bool
		arranqueOK   bool
		hayActividad bool
		quiero       time.Duration
	}{
		{"arranque fallo: primer tick corto", true, false, false, 1 * time.Minute},
		{"arranque ok: 15 min", true, true, false, 15 * time.Minute},
		{"arranque fallo pero ya no es el primero: 15 min", false, false, false, 15 * time.Minute},
		{"hay partido LIVE: 1 min", false, true, true, 1 * time.Minute},
		{"hay partido en <30 min: 1 min", true, true, true, 1 * time.Minute},
		{"arranque fallo Y hay actividad: 1 min", true, false, true, 1 * time.Minute},
	}
	for _, c := range casos {
		got := intervaloDelLoop(c.primero, c.arranqueOK, c.hayActividad)
		if got != c.quiero {
			t.Errorf("%s: dio %v, quiero %v", c.nombre, got, c.quiero)
		}
	}
}

// Y que el ciclo normal no se haya acortado sin querer: 15 min es el default del diseño.
func TestElDefaultSigueSiendo15Minutos(t *testing.T) {
	if got := intervaloDelLoop(false, true, false); got != 15*time.Minute {
		t.Errorf("el default dio %v, quiero 15m", got)
	}
}
