package finance

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type DolarPrice struct {
	Compra float64 `json:"compra"`
	Venta  float64 `json:"venta"`
	Nombre string  `json:"nombre"`
}

type FinanceData struct {
	Blue   DolarPrice
	Cripto DolarPrice
}

var (
	cachedFinance FinanceData
	financeMutex  sync.RWMutex
)

func init() {}

func StartUpdateLoop() {
	go func() {
		for {
			UpdateDollar()
			time.Sleep(15 * time.Minute)
		}
	}()
}

func UpdateDollar() error {
	var newData FinanceData

	// Usamos DolarApi.com para el Blue
	respB, errB := http.Get("https://dolarapi.com/v1/dolares/blue")
	if errB == nil {
		defer respB.Body.Close()
		var d DolarPrice
		if err := json.NewDecoder(respB.Body).Decode(&d); err == nil && d.Venta > 0 {
			newData.Blue = d
		}
	}

	// Usamos DolarApi.com para el Cripto
	respC, errC := http.Get("https://dolarapi.com/v1/dolares/cripto")
	if errC == nil {
		defer respC.Body.Close()
		var d DolarPrice
		if err := json.NewDecoder(respC.Body).Decode(&d); err == nil && d.Venta > 0 {
			newData.Cripto = d
		}
	}

	if newData.Blue.Venta == 0 && newData.Cripto.Venta == 0 {
		return http.ErrHandlerTimeout // O cualquier error para indicar fallo
	}

	financeMutex.Lock()
	if newData.Blue.Venta > 0 { cachedFinance.Blue = newData.Blue }
	if newData.Cripto.Venta > 0 { cachedFinance.Cripto = newData.Cripto }
	financeMutex.Unlock()
	return nil
}

func GetCachedFinance() FinanceData {
	financeMutex.RLock()
	defer financeMutex.RUnlock()
	return cachedFinance
}
