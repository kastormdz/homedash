package holidays

import (
	"fmt"
	"homedash/internal/common"
	"sync"
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
}

// 5. Cache de feriados por año para evitar recálculo en cada request
var (
	cachedHolidays    []Holiday
	cachedHolidayYear int
	holidayCacheMutex sync.RWMutex
)

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

func GetArgentinaHolidays() []Holiday {
	year := time.Now().Year()

	// 5. Cache: devolver si ya calculamos para este año
	holidayCacheMutex.RLock()
	if cachedHolidayYear == year && cachedHolidays != nil {
		result := cachedHolidays
		holidayCacheMutex.RUnlock()
		return result
	}
	holidayCacheMutex.RUnlock()

	yStr := fmt.Sprintf("%d", year)

	// Feriados inamovibles (Día/Mes)
	fixed := []struct{ Name, Date string }{
		{"Año Nuevo", "01-01"},
		{"Día del Veterano", "04-02"},
		{"Día del Trabajador", "05-01"},
		{"Revolución de Mayo", "05-25"},
		{"Día de la Independencia", "07-09"},
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
		{"Gral. Belgrano", fmt.Sprintf("%04d-06-20", year)},
		{"Gral. San Martín", fmt.Sprintf("%04d-08-17", year)},
		{"Diversidad Cultural", fmt.Sprintf("%04d-10-12", year)},
	}
	for _, h := range trasladables {
		t, err := time.Parse("2006-01-02", h.Date)
		if err == nil {
			moved := nextMonday(t)
			list = append(list, Holiday{Name: h.Name, Date: moved.Format("2006-01-02")})
		}
	}

	// Feriados turísticos inamovibles
	turisticos := []struct{ Name, Date string }{
		{"Feriado Turístico", fmt.Sprintf("%04d-03-24", year)},
		{"Día de la Memoria", fmt.Sprintf("%04d-03-24", year)},
		{"Feriado Turístico", fmt.Sprintf("%04d-07-09", year)},
		{"Soberanía Nacional", fmt.Sprintf("%04d-11-20", year)},
		{"Feriado Turístico", fmt.Sprintf("%04d-12-08", year)},
	}
	for _, h := range turisticos {
		list = append(list, Holiday{Name: h.Name, Date: h.Date})
	}

	// 5. Guardar en cache
	holidayCacheMutex.Lock()
	cachedHolidays = list
	cachedHolidayYear = year
	holidayCacheMutex.Unlock()

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

	for _, h := range GetArgentinaHolidays() {
		if h.Date > todayStr && h.Date <= laterStr {
			d, _ := time.Parse("2006-01-02", h.Date)
			upcoming = append(upcoming, UpcomingHoliday{
				Name:    h.Name,
				DayName: common.DaysAbbr[d.Weekday()],
				DateStr: d.Format("02/01"),
			})
		}
	}
	return upcoming
}
