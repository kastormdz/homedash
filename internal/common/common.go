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
	"'", "", "'", "", " ", "", "(", "", ")", "",
	".", "", "-", "", ",", "",
)

func NormalizeName(name string) string {
	return normalizer.Replace(strings.ToLower(name))
}
