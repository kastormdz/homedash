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
		{Fondo: "NARANJA", TNA: 0.18, Fecha: hoy},
		{Fondo: "SIN TNA", TNA: 0, Fecha: hoy},
	}
	top := topBilleteras(fondos)
	// Solo limpias (sin tramos ni condiciones), ordenadas por TNA.
	esperado := []Billetera{
		{Fondo: "BELO", TNA: 19},
		{Fondo: "NARANJA", TNA: 18},
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

func TestTopBilleterasMaxCinco(t *testing.T) {
	hoy := time.Now().Format("2006-01-02")
	var fondos []fondoFCI
	for i := 0; i < 8; i++ {
		fondos = append(fondos, fondoFCI{Fondo: string(rune('A' + i)), TNA: 0.10 + float64(i)/100, Fecha: hoy})
	}
	top := topBilleteras(fondos)
	if len(top) != 5 {
		t.Fatalf("largo = %d, quiero 5", len(top))
	}
	if top[0].TNA != 17 {
		t.Errorf("primera TNA = %v, quiero 17", top[0].TNA)
	}
}

func fondoCT(nombre, adm, moneda, tipo, fecha string, tna, patrimonio float64) fondoComparaTasas {
	f := fondoComparaTasas{Nombre: nombre, Administradora: adm, Moneda: moneda, TipoRenta: tipo, Fecha: fecha, Patrimonio: patrimonio}
	f.Rendimientos.TNAEstimada = tna
	return f
}

func TestMercadoPagoDe(t *testing.T) {
	hoy := time.Now().Format("2006-01-02")
	fondos := []fondoComparaTasas{
		fondoCT("Adcap Ahorro Pesos", "Adcap Asset Management S.G.F.C.I.S.A.", "Peso Argentino", "Mercado de Dinero", hoy, 19.88, 4e10),
		// Mercado Pago: la cuenta remunerada es el money market en pesos.
		fondoCT("Mercado Fondo - Clase C", "Mercado Pago Asset Management S.A.", "Peso Argentino", "Mercado de Dinero", hoy, 19.1, 2.5e11),
		fondoCT("Mercado Fondo - Clase A", "Mercado Pago Asset Managemet S.A.", "Peso Argentino", "Mercado de Dinero", hoy, 19.1469, 9.8e12),
		// No aplican: dólares y renta fija de la misma administradora.
		fondoCT("Mercado Fondo USD", "Mercado Pago Asset Management S.A.", "Dolar Estadounidense", "Mercado de Dinero", hoy, 1.5, 1e10),
		fondoCT("MP Ahorro - Clase A", "Mercado Pago Asset Management S.A.", "Peso Argentino", "Renta Fija", hoy, 30.0186, 5e10),
	}
	mp, ok := mercadoPagoDe(fondos)
	if !ok {
		t.Fatal("no encontró Mercado Pago")
	}
	if mp.Fondo != "MERCADO PAGO" || mp.TNA != 19.1469 {
		t.Errorf("mp = %+v, quiero MERCADO PAGO 19.1469", mp)
	}
}

func TestMercadoPagoDeSinDatos(t *testing.T) {
	hoy := time.Now().Format("2006-01-02")
	vieja := time.Now().AddDate(0, 0, -60).Format("2006-01-02")
	casos := [][]fondoComparaTasas{
		nil,
		{fondoCT("Mercado Fondo - Clase A", "Mercado Pago Asset Management S.A.", "Peso Argentino", "Mercado de Dinero", vieja, 19.1, 9.8e12)},
		{fondoCT("Mercado Fondo - Clase A", "Mercado Pago Asset Management S.A.", "Peso Argentino", "Mercado de Dinero", hoy, 0, 9.8e12)},
	}
	for i, fondos := range casos {
		if _, ok := mercadoPagoDe(fondos); ok {
			t.Errorf("caso %d: no debería devolver Mercado Pago", i)
		}
	}
}

func TestRecortarTop(t *testing.T) {
	lista := []Billetera{
		{Fondo: "A", TNA: 17},
		{Fondo: "MERCADO PAGO", TNA: 19.1},
		{Fondo: "B", TNA: 20},
	}
	top := recortarTop(lista, 2)
	if len(top) != 2 || top[0].Fondo != "B" || top[1].Fondo != "MERCADO PAGO" {
		t.Errorf("top = %+v", top)
	}
}
