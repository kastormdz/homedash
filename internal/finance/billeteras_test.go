package finance

import (
	"testing"
	"time"
)

func TestTopBilleteras(t *testing.T) {
	hoy := time.Now().Format("2006-01-02")
	vieja := time.Now().AddDate(0, 0, -60).Format("2006-01-02")
	fondos := []fondoFCI{
		{Fondo: "BELO", TNA: 0.19, Fecha: hoy},
		{Fondo: "BRUBANK", TNA: 0.27, Fecha: hoy}, // fee mensual: afuera igual
		{Fondo: "VIEJO", TNA: 0.99, Fecha: vieja}, // rancio: afuera
		{Fondo: "UALA", TNA: 0.19, Tope: 1000000, Fecha: hoy},
		{Fondo: "UALA PLUS 2", TNA: 0.24, Tope: 1000000, Condiciones: "Si sumás $500.000", Fecha: hoy},
		{Fondo: "BICA CUENTA POSITIVA 4", TNA: 0.22, Tope: 750000, Condiciones: "de $1 hasta $750.000", Fecha: hoy},
		{Fondo: "BICA CUENTA POSITIVA 2", TNA: 0.16, Tope: 20000000, Fecha: hoy},
		{Fondo: "SIN TNA", TNA: 0, Fecha: hoy},
	}
	top := topBilleteras(fondos)
	// Limpias: BELO. Relleno por banco con base: UALA 19*, BICA 16*. Orden global.
	esperado := []Billetera{
		{Fondo: "BELO", TNA: 19},
		{Fondo: "UALA", TNA: 19, HasConditions: true},
		{Fondo: "BICA", TNA: 16, HasConditions: true},
	}
	if len(top) != len(esperado) {
		t.Fatalf("largo = %d, quiero %d (%v)", len(top), len(esperado), top)
	}
	for i, q := range esperado {
		if top[i] != q {
			t.Errorf("pos %d = %+v, quiero %+v", i, top[i], q)
		}
	}
}
