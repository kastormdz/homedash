package holidays

import (
	"fmt"
	"homedash/internal/common"
	"sort"
	"time"
)

type Holiday struct {
	Name string
	Date string // Formato "2006-01-02"
}

// Para la UI
type UpcomingHoliday struct {
	Name    string
	DayName string
	DateStr string
	IsNear  bool // Próximos 2 días
}

// R-A. Calcular Domingo de Pascuas (algoritmo de Computus de Meeus/Jones/Butcher)
func easterSunday(year int) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := ((h + l - 7*m + 114) % 31) + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
}

// lunesSegunLey aplica el art. 6 de la ley 27.399 para los feriados trasladables:
//
//	martes y miercoles  -> lunes ANTERIOR
//	jueves y viernes    -> lunes SIGUIENTE
//	sabado y domingo    -> lunes siguiente (no pasa en la practica)
//
// Antes se usaba nextMonday para todo, y en 2026 el 17/06 (miercoles) caia el 22/06,
// el mismo dia que Belgrano: dos feriados pisados y uno invisible en el panel.
func lunesSegunLey(t time.Time) time.Time {
	switch t.Weekday() {
	case time.Monday:
		return t
	case time.Tuesday, time.Wednesday:
		// retroceder al lunes anterior: martes -1, miercoles -2
		return t.AddDate(0, 0, 1-int(t.Weekday()))
	default:
		return nextMonday(t)
	}
}

// nextMonday devuelve el próximo lunes a partir de la fecha dada (inclusive)
func nextMonday(t time.Time) time.Time {
	if t.Weekday() == time.Monday {
		return t
	}
	daysUntilMonday := (int(time.Monday) - int(t.Weekday()) + 7) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}
	return t.AddDate(0, 0, daysUntilMonday)
}

// calcularFeriados es el FALLBACK: el calendario calculado a mano, sin red.
//
// La fuente primaria es la API (ver api.go). Esto queda para cuando la API no responde:
// el panel no puede quedarse sin feriados porque un tercero este caido.
//
// OJO con la ley 27.399 art. 6: los trasladables que caen MARTES o MIERCOLES van al lunes
// ANTERIOR; los que caen jueves o viernes, al lunes siguiente. Antes esto movia todo al
// lunes siguiente, y el 17/06/2026 (miercoles) se iba al 22/06, el mismo dia que Belgrano.
func calcularFeriados(year int) []Holiday {
	yStr := fmt.Sprintf("%d", year)

	// Feriados inamovibles (Día/Mes)
	fixed := []struct{ Name, Date string }{
		{"Año Nuevo", "01-01"},
		{"Día de la Memoria", "03-24"},
		{"Día del Veterano", "04-02"},
		{"Día del Trabajador", "05-01"},
		{"Revolución de Mayo", "05-25"},
		{"Día de la Independencia", "07-09"},
		{"Gral. Belgrano", "06-20"},
		{"Inmaculada Concepción", "12-08"},
		{"Navidad", "12-25"},
	}

	var list []Holiday
	for _, h := range fixed {
		list = append(list, Holiday{Name: h.Name, Date: yStr + "-" + h.Date})
	}

	// R-A. Feriados móviles calculados algorítmicamente
	easter := easterSunday(year)

	// Carnaval: 48 y 47 días antes de Pascuas
	carnaval1 := easter.AddDate(0, 0, -48)
	carnaval2 := easter.AddDate(0, 0, -47)
	list = append(list,
		Holiday{Name: "Carnaval", Date: carnaval1.Format("2006-01-02")},
		Holiday{Name: "Carnaval", Date: carnaval2.Format("2006-01-02")},
	)

	// Viernes Santo: 2 días antes de Pascuas
	viernesSanto := easter.AddDate(0, 0, -2)
	list = append(list, Holiday{Name: "Viernes Santo", Date: viernesSanto.Format("2006-01-02")})

	// Feriados trasladables (se mueven al lunes más cercano)
	trasladables := []struct{ Name, Date string }{
		{"Gral. Güemes", fmt.Sprintf("%04d-06-17", year)},
		{"Gral. San Martín", fmt.Sprintf("%04d-08-17", year)},
		{"Diversidad Cultural", fmt.Sprintf("%04d-10-12", year)},
		// Soberanía Nacional es TRASLADABLE (20/11): antes estaba como si fuera fijo.
		{"Soberanía Nacional", fmt.Sprintf("%04d-11-20", year)},
	}
	for _, h := range trasladables {
		t, err := time.Parse("2006-01-02", h.Date)
		if err == nil {
			moved := lunesSegunLey(t)
			list = append(list, Holiday{Name: h.Name, Date: moved.Format("2006-01-02")})
		}
	}

	// Feriados turísticos específicos por año (Solo los que no están en 'fixed' o son móviles)
	// Los feriados turisticos (art. 7, hasta 3 por año, los fija el PEN por decreto) no se
	// pueden calcular: los trae la API. Aca no hay nada que agregar.

	// Ordenamos la lista por fecha para asegurar consistencia
	sort.Slice(list, func(i, j int) bool {
		return list[i].Date < list[j].Date
	})

	return list
}

func GetHolidayToday(t time.Time) *Holiday {
	todayStr := t.Format("2006-01-02")
	for _, h := range GetArgentinaHolidays() {
		if h.Date == todayStr {
			return &h
		}
	}
	return nil
}

func GetUpcomingHolidays(t time.Time) []UpcomingHoliday {
	var upcoming []UpcomingHoliday
	todayStr := t.Format("2006-01-02")
	later := t.AddDate(0, 0, 30)
	laterStr := later.Format("2006-01-02")

	// Umbral para "IsNear" (hoy + 5 días)
	nearThreshold := t.AddDate(0, 0, 5).Format("2006-01-02")

	for _, h := range GetArgentinaHolidays() {
		if h.Date > todayStr && h.Date <= laterStr {
			d, _ := time.Parse("2006-01-02", h.Date)
			upcoming = append(upcoming, UpcomingHoliday{
				Name:    h.Name,
				DayName: common.DaysAbbr[d.Weekday()],
				DateStr: d.Format("02/01"),
				IsNear:  h.Date <= nearThreshold,
			})
		}
	}
	return upcoming
}
