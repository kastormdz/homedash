package common

import (
	"strings"
	"time"
)

var DaysAbbr = map[time.Weekday]string{
	time.Sunday:    "Dom",
	time.Monday:    "Lun",
	time.Tuesday:   "Mar",
	time.Wednesday: "Mié",
	time.Thursday:  "Jue",
	time.Friday:    "Vie",
	time.Saturday:  "Sáb",
}

// R-B. Usar strings.NewReplacer en vez de 8 llamadas a ReplaceAll
var normalizer = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u",
	"ä", "a", "ë", "e", "ï", "i", "ö", "o", "ü", "u",
	"â", "a", "ê", "e", "î", "i", "ô", "o", "û", "u",
	"ã", "a", "õ", "o", "ñ", "n",
	"'", "", " ", "", "(", "", ")", "",
	".", "", "-", "", ",", "",
)

func NormalizeName(name string) string {
	lower := strings.ToLower(name)
	lower = strings.ReplaceAll(lower, " de ", " ")
	lower = strings.ReplaceAll(lower, " del ", " ")
	lower = strings.ReplaceAll(lower, "santiago estero", "sde")
	return normalizer.Replace(lower)
}
