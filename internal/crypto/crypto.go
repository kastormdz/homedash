package crypto

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var (
	cachedBTC       float64
	cachedBTCChange float64
	btcMutex        sync.RWMutex
)

func init() {}

func StartUpdateLoop() {
	go func() {
		for {
			UpdateBTC()
			time.Sleep(2 * time.Minute)
		}
	}()
}

func UpdateBTC() error {
	price, change, err := GetBTCData()
	if err != nil || price == 0 {
		return fmt.Errorf("error obteniendo BTC")
	}

	btcMutex.Lock()
	cachedBTC = price
	cachedBTCChange = change
	btcMutex.Unlock()
	return nil
}

func GetCachedBTC() float64 {
	btcMutex.RLock()
	defer btcMutex.RUnlock()
	return cachedBTC
}

func GetCachedBTCChange() float64 {
	btcMutex.RLock()
	defer btcMutex.RUnlock()
	return cachedBTCChange
}

type CoinGeckoResponse map[string]struct {
	USD       float64 `json:"usd"`
	USDChange float64 `json:"usd_24h_change"`
}

type BinanceResponse struct {
	Price string `json:"price"`
}

type Binance24hResponse struct {
	PriceChangePercent string `json:"priceChangePercent"`
}

func GetBTCData() (float64, float64, error) {
	// Fuente 1: CoinGecko (incluye % 24h)
	resp, err := http.Get("https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd&include_24hr_change=true")
	if err == nil {
		defer resp.Body.Close()
		var result CoinGeckoResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result["bitcoin"].USD > 0 {
			return result["bitcoin"].USD, result["bitcoin"].USDChange, nil
		}
	}

	// Fuente 2: Binance (Respaldo)
	var price, change float64
	resp2, err := http.Get("https://api.binance.com/api/v3/ticker/price?symbol=BTCUSDT")
	if err == nil {
		defer resp2.Body.Close()
		var result BinanceResponse
		if err := json.NewDecoder(resp2.Body).Decode(&result); err == nil {
			price, _ = strconv.ParseFloat(result.Price, 64)
		}
	}

	resp3, err := http.Get("https://api.binance.com/api/v3/ticker/24hr?symbol=BTCUSDT")
	if err == nil {
		defer resp3.Body.Close()
		var result Binance24hResponse
		if err := json.NewDecoder(resp3.Body).Decode(&result); err == nil {
			change, _ = strconv.ParseFloat(result.PriceChangePercent, 64)
		}
	}

	if price > 0 {
		return price, change, nil
	}

	return 0, 0, fmt.Errorf("no se pudo obtener precio de BTC de ninguna fuente")
}
