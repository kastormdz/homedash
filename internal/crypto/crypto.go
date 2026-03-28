package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"homedash/internal/network"
	"strconv"
	"sync"
	"time"
)

var (
	cachedBTC       float64
	cachedBTCChange float64
	cachedETH       float64
	cachedETHChange float64
	cryptoMutex     sync.RWMutex
)

func StartUpdateLoop() {
	go func() {
		for {
			// P-3. Contexto con timeout para evitar bloqueos infinitos
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = UpdateCrypto(ctx)
			cancel()
			time.Sleep(2 * time.Minute)
		}
	}()
}

func UpdateCrypto(ctx context.Context) error {
	var wg sync.WaitGroup
	wg.Add(2)

	var btcPrice, btcChange float64
	var ethPrice, ethChange float64
	var err1, err2 error

	go func() {
		defer wg.Done()
		btcPrice, btcChange, err1 = GetCryptoData(ctx, "bitcoin", "BTCUSDT")
	}()

	go func() {
		defer wg.Done()
		ethPrice, ethChange, err2 = GetCryptoData(ctx, "ethereum", "ETHUSDT")
	}()

	wg.Wait()

	if err1 != nil && err2 != nil {
		return fmt.Errorf("error obteniendo criptos: %v, %v", err1, err2)
	}

	cryptoMutex.Lock()
	if err1 == nil && btcPrice > 0 {
		cachedBTC = btcPrice
		cachedBTCChange = btcChange
	}
	if err2 == nil && ethPrice > 0 {
		cachedETH = ethPrice
		cachedETHChange = ethChange
	}
	cryptoMutex.Unlock()
	return nil
}

func GetCachedBTC() float64 {
	cryptoMutex.RLock()
	defer cryptoMutex.RUnlock()
	return cachedBTC
}

func GetCachedBTCChange() float64 {
	cryptoMutex.RLock()
	defer cryptoMutex.RUnlock()
	return cachedBTCChange
}

func GetCachedETH() float64 {
	cryptoMutex.RLock()
	defer cryptoMutex.RUnlock()
	return cachedETH
}

func GetCachedETHChange() float64 {
	cryptoMutex.RLock()
	defer cryptoMutex.RUnlock()
	return cachedETHChange
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

func GetCryptoData(ctx context.Context, cgID, binanceSymbol string) (float64, float64, error) {
	// Fuente 1: CoinGecko (incluye % 24h)
	urlCG := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd&include_24hr_change=true", cgID)
	resp, err := network.FetchSecureWithContext(ctx, urlCG)
	if err == nil {
		defer resp.Body.Close()
		var result CoinGeckoResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result[cgID].USD > 0 {
			return result[cgID].USD, result[cgID].USDChange, nil
		}
	}

	// Fuente 2: Binance (Respaldo) — P-C. Paralelizar precio + stats 24h
	var price, change float64
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		resp2, err := network.FetchSecureWithContext(ctx, "https://api.binance.com/api/v3/ticker/price?symbol="+binanceSymbol)
		if err != nil {
			return
		}
		defer resp2.Body.Close()
		var result BinanceResponse
		if err := json.NewDecoder(resp2.Body).Decode(&result); err == nil {
			price, _ = strconv.ParseFloat(result.Price, 64)
		}
	}()

	go func() {
		defer wg.Done()
		resp3, err := network.FetchSecureWithContext(ctx, "https://api.binance.com/api/v3/ticker/24hr?symbol="+binanceSymbol)
		if err != nil {
			return
		}
		defer resp3.Body.Close()
		var result Binance24hResponse
		if err := json.NewDecoder(resp3.Body).Decode(&result); err == nil {
			change, _ = strconv.ParseFloat(result.PriceChangePercent, 64)
		}
	}()

	wg.Wait()

	if price > 0 {
		return price, change, nil
	}

	return 0, 0, fmt.Errorf("no se pudo obtener precio de %s", binanceSymbol)
}
