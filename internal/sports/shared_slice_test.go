package sports

import (
	"sync"
	"testing"
)

// BUG: GetSportsData() devuelve cachedData por valor, pero AllMatches es un []MatchData.
// Copiar el struct copia el puntero del slice, NO los datos: todos los lectores comparten
// el mismo backing array. Cualquier handler que modifique un elemento (los escudos de
// /partidos-del-dia, el TeamCrest del partido destacado) escribe en el array compartido
// mientras el updater en background puede estar reescribiendolo -> data race y
// potencialmente un "slice bounds out of range".
//
// Este test lo demuestra de forma determinista: N lectores tocan el slice mientras un
// escritor lo modifica, bajo -race. Sin el fix, el detector de Go lo reporta.
func TestGetSportsDataNoComparteElSlice(t *testing.T) {
	// sembrar datos
	cacheMutex.Lock()
	cachedData = SportsData{
		AllMatches: []MatchData{
			{Team: "Boca Juniors", Opponent: "River"},
			{Team: "Racing Club", Opponent: "Independiente"},
		},
	}
	cacheMutex.Unlock()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// lectores: tocan los elementos como hacen los handlers
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					for _, m := range GetSportsData().AllMatches {
						_ = m.Team
						_ = m.OpponentCrest
					}
				}
			}
		}()
	}

	// escritor: el updater reescribe los elementos, como hace fetchFreshSportsData
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				cacheMutex.Lock()
				for i := range cachedData.AllMatches {
					cachedData.AllMatches[i].TeamCrest = "/static/x.png"
					cachedData.AllMatches[i].Status = "LIVE"
				}
				cacheMutex.Unlock()
			}
		}
	}()

	// dejar correr un rato y frenar
	for i := 0; i < 20000; i++ {
		_ = GetSportsData().AllMatches
	}
	close(stop)
	wg.Wait()
}

// Test estructural: el slice devuelto no debe ser el mismo backing array que el interno.
// Si se cumple, un handler que modifique lo que recibio NO toca el estado global.
func TestGetSportsDataDevuelveUnaCopia(t *testing.T) {
	cacheMutex.Lock()
	original := []MatchData{{Team: "Boca Juniors"}, {Team: "Racing Club"}}
	cachedData = SportsData{AllMatches: original}
	cacheMutex.Unlock()

	copia := GetSportsData()
	copia.AllMatches[0].Team = "MODIFICADO"

	otro := GetSportsData()
	if otro.AllMatches[0].Team == "MODIFICADO" {
		t.Fatal("el estado global fue mutado a traves del valor devuelto: se comparte el slice")
	}
	if otro.AllMatches[0].Team != "Boca Juniors" {
		t.Fatalf("el estado global quedo corrupto: %q", otro.AllMatches[0].Team)
	}
}