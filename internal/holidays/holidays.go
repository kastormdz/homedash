package holidays

import (
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

func GetArgentinaHolidays2026() []Holiday {
	return []Holiday{
		{Name: "Año Nuevo", Date: "2026-01-01"},
		{Name: "Carnaval", Date: "2026-02-16"},
		{Name: "Carnaval", Date: "2026-02-17"},
		{Name: "Feriado Turístico", Date: "2026-03-23"},
		{Name: "Día de la Memoria", Date: "2026-03-24"},
		{Name: "Viernes Santo", Date: "2026-04-03"},
		{Name: "Día del Veterano", Date: "2026-04-02"},
		{Name: "Día del Trabajador", Date: "2026-05-01"},
		{Name: "Revolución de Mayo", Date: "2026-05-25"},
		{Name: "Gral. Güemes", Date: "2026-06-15"},
		{Name: "Gral. Belgrano", Date: "2026-06-20"},
		{Name: "Día de la Independencia", Date: "2026-07-09"},
		{Name: "Feriado Turístico", Date: "2026-07-10"},
		{Name: "Gral. San Martín", Date: "2026-08-17"},
		{Name: "Diversidad Cultural", Date: "2026-10-12"},
		{Name: "Soberanía Nacional", Date: "2026-11-23"},
		{Name: "Feriado Turístico", Date: "2026-12-07"},
		{Name: "Inmaculada Concepción", Date: "2026-12-08"},
		{Name: "Navidad", Date: "2026-12-25"},
	}
}

func GetHolidayToday(t time.Time) *Holiday {
	todayStr := t.Format("2006-01-02")
	for _, h := range GetArgentinaHolidays2026() {
		if h.Date == todayStr {
			return &h
		}
	}
	return nil
}

func GetUpcomingHolidays(t time.Time) []UpcomingHoliday {
	var upcoming []UpcomingHoliday
	days := map[string]string{
		"Monday": "Lun", "Tuesday": "Mar", "Wednesday": "Mié",
		"Thursday": "Jue", "Friday": "Vie", "Saturday": "Sáb", "Sunday": "Dom",
	}

	for i := 1; i <= 30; i++ {
		nextDay := t.AddDate(0, 0, i)
		if h := GetHolidayToday(nextDay); h != nil {
			upcoming = append(upcoming, UpcomingHoliday{
				Name:    h.Name,
				DayName: days[nextDay.Weekday().String()],
				DateStr: nextDay.Format("02/01"),
			})
		}
	}
	return upcoming
}
