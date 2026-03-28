package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"homedash/internal/network"
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
	Blue   DolarPrice
	Cripto DolarPrice
	SP500  AssetData
	Nasdaq AssetData
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
	wg.Add(4)

	// P-D. Paralelizar Blue y Cripto
	go func() {
		defer wg.Done()
		respB, errB := network.FetchSecureWithContext(ctx, "https://dolarapi.com/v1/dolares/blue")
		if errB == nil {
			defer respB.Body.Close()
			var d DolarPrice
			if err := json.NewDecoder(respB.Body).Decode(&d); err == nil && d.Venta > 0 {
				newData.Blue = d
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
				newData.Cripto = d
			}
		}
	}()

	// P-E. Paralelizar S&P500 y Nasdaq
	go func() {
		defer wg.Done()
		if spyData, err := getYahooFinanceData(ctx, "^GSPC"); err == nil && spyData.Price > 0 {
			newData.SP500 = spyData
		}
	}()

	go func() {
		defer wg.Done()
		if qqqData, err := getYahooFinanceData(ctx, "^IXIC"); err == nil && qqqData.Price > 0 {
			newData.Nasdaq = qqqData
		}
	}()

	wg.Wait()

	if newData.Blue.Venta == 0 && newData.Cripto.Venta == 0 && newData.SP500.Price == 0 && newData.Nasdaq.Price == 0 {
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
	financeMutex.Unlock()
	return nil
}

func GetCachedFinance() FinanceData {
	financeMutex.RLock()
	defer financeMutex.RUnlock()
	return cachedFinance
}
