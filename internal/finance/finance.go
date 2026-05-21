package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"homedash/internal/network"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DolarPrice struct {
	Compra float64 `json:"compra"`
	Venta  float64 `json:"venta"`
	Nombre string  `json:"nombre"`
}

type AssetData struct {
	Price  float64
	Change float64
}

type FinanceData struct {
	Blue       DolarPrice
	Cripto     DolarPrice
	SP500      AssetData
	Nasdaq     AssetData
	RiesgoPais AssetData
}

var (
	cachedFinance FinanceData
	financeMutex  sync.RWMutex
)

func StartUpdateLoop() {
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = UpdateFinance(ctx)
			cancel()
			time.Sleep(5 * time.Minute)
		}
	}()
}

type YahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				ChartPreviousClose float64 `json:"chartPreviousClose"`
			} `json:"meta"`
		} `json:"result"`
	} `json:"chart"`
}

func getYahooFinanceData(ctx context.Context, symbol string) (AssetData, error) {
	resp, err := network.FetchSecureWithContext(ctx, "https://query1.finance.yahoo.com/v8/finance/chart/"+symbol)
	if err != nil {
		return AssetData{}, err
	}
	defer resp.Body.Close()

	var data YahooChartResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return AssetData{}, err
	}

	if len(data.Chart.Result) > 0 {
		meta := data.Chart.Result[0].Meta
		var change float64
		if meta.ChartPreviousClose > 0 {
			change = ((meta.RegularMarketPrice - meta.ChartPreviousClose) / meta.ChartPreviousClose) * 100
		}
		return AssetData{
			Price:  meta.RegularMarketPrice,
			Change: change,
		}, nil
	}
	return AssetData{}, fmt.Errorf("no data for symbol")
}

func UpdateFinance(ctx context.Context) error {
	var newData FinanceData
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(5)

	// P-D. Paralelizar Blue, Cripto y Riesgo País
	go func() {
		defer wg.Done()
		respB, errB := network.FetchSecureWithContext(ctx, "https://dolarapi.com/v1/dolares/blue")
		if errB == nil {
			defer respB.Body.Close()
			var d DolarPrice
			if err := json.NewDecoder(respB.Body).Decode(&d); err == nil && d.Venta > 0 {
				mu.Lock()
				newData.Blue = d
				mu.Unlock()
			}
		}
	}()

	go func() {
		defer wg.Done()
		respC, errC := network.FetchSecureWithContext(ctx, "https://dolarapi.com/v1/dolares/cripto")
		if errC == nil {
			defer respC.Body.Close()
			var d DolarPrice
			if err := json.NewDecoder(respC.Body).Decode(&d); err == nil && d.Venta > 0 {
				mu.Lock()
				newData.Cripto = d
				mu.Unlock()
			}
		}
	}()

	go func() {
		defer wg.Done()
		
		type rpResult struct {
			val    float64
			change float64
		}
		resCh := make(chan rpResult, 2)
		var rpWg sync.WaitGroup
		rpWg.Add(2)

		go func() {
			defer rpWg.Done()
			// Ámbito requiere User-Agent y a veces otros headers, FetchSecureWithContext los provee.
			respA, errA := network.FetchSecureWithContext(ctx, "https://mercados.ambito.com/riesgopais/variacion")
			if errA == nil {
				defer respA.Body.Close()
				var a struct {
					Ultimo    string `json:"ultimo"`
					Variacion string `json:"variacion"`
				}
				if err := json.NewDecoder(respA.Body).Decode(&a); err == nil && a.Ultimo != "" {
					val, _ := strconv.ParseFloat(strings.ReplaceAll(a.Ultimo, ",", "."), 64)
					varStr := strings.TrimSuffix(a.Variacion, "%")
					change, _ := strconv.ParseFloat(strings.ReplaceAll(varStr, ",", "."), 64)
					if val > 0 {
						resCh <- rpResult{val, change}
						return
					}
				}
			} else {
				log.Printf("[FINANCE] Error Ámbito: %v", errA)
			}
		}()

		go func() {
			defer rpWg.Done()
			if respR, errR := network.FetchSecureWithContext(ctx, "https://api.argentinadatos.com/v1/finanzas/indices/riesgo-pais"); errR == nil {
				defer respR.Body.Close()
				var history []struct {
					Valor float64 `json:"valor"`
					Fecha string  `json:"fecha"`
				}
				if err := json.NewDecoder(respR.Body).Decode(&history); err == nil && len(history) > 1 {
					last := history[len(history)-1]
					prev := history[len(history)-2]
					change := 0.0
					if prev.Valor > 0 {
						change = ((last.Valor - prev.Valor) / prev.Valor) * 100
					}
					if last.Valor > 0 {
						resCh <- rpResult{last.Valor, change}
						return
					}
				}
			} else {
				log.Printf("[FINANCE] Error ArgentinaDatos: %v", errR)
			}
		}()

		go func() {
			rpWg.Wait()
			close(resCh)
		}()

		if res, ok := <-resCh; ok {
			mu.Lock()
			newData.RiesgoPais = AssetData{Price: res.val, Change: res.change}
			mu.Unlock()
		}
	}()

	// P-E. Paralelizar S&P500 y Nasdaq
	go func() {
		defer wg.Done()
		if spyData, err := getYahooFinanceData(ctx, "^GSPC"); err == nil && spyData.Price > 0 {
			mu.Lock()
			newData.SP500 = spyData
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		if qqqData, err := getYahooFinanceData(ctx, "^IXIC"); err == nil && qqqData.Price > 0 {
			mu.Lock()
			newData.Nasdaq = qqqData
			mu.Unlock()
		}
	}()

	wg.Wait()

	if newData.Blue.Venta == 0 && newData.Cripto.Venta == 0 && newData.SP500.Price == 0 && newData.Nasdaq.Price == 0 && newData.RiesgoPais.Price == 0 {
		return fmt.Errorf("no se pudieron obtener datos financieros")
	}

	financeMutex.Lock()
	if newData.Blue.Venta > 0 {
		cachedFinance.Blue = newData.Blue
	}
	if newData.Cripto.Venta > 0 {
		cachedFinance.Cripto = newData.Cripto
	}
	if newData.SP500.Price > 0 {
		cachedFinance.SP500 = newData.SP500
	}
	if newData.Nasdaq.Price > 0 {
		cachedFinance.Nasdaq = newData.Nasdaq
	}
	if newData.RiesgoPais.Price > 0 {
		cachedFinance.RiesgoPais = newData.RiesgoPais
	}
	financeMutex.Unlock()
	return nil
}

func GetCachedFinance() FinanceData {
	financeMutex.RLock()
	defer financeMutex.RUnlock()
	return cachedFinance
}
