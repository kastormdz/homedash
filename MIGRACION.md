# MIGRACION.md — Auditoría Homedash (v2)

Análisis del proyecto después de los cambios aplicados. Se documenta lo que ya fue resuelto y lo que queda pendiente.

---

## Estado de resolución (Auditoría v1)

| Issue | Estado | Notas |
|-------|--------|-------|
| S1. SSRF proxy de escudos | ✅ Resuelto | Nuevo paquete `network` con `IsDomainAllowed` allowlist |
| S2. XSS en autocomplete | ✅ Resuelto | `template.HTMLEscapeString()` aplicado |
| S3. Cookie sin Secure/SameSite | ✅ Resuelto | Flags agregados, `Secure` dinámico según TLS |
| S4. Tema sin validar | ✅ Resuelto | Allowlist de 9 temas |
| S4. Coordenadas sin validar | ✅ Resuelto | Rango -90/90 y -180/180 verificado |
| B1. Panic nil dereference proxy | ✅ Resuelto | `FetchSecure` encapsula validación err→status |
| B3. http.NewRequest ignorado | ✅ Resuelto | `FetchSecure` maneja el error |
| B5. strings.Title deprecado | ✅ Resuelto | `cases.Title(language.Spanish)` |
| RC1. TOCTOU weather cache | ✅ Resuelto | Double-check locking implementado |
| RC2. Race alert cache | ✅ Resuelto | `findAlertInCache` evaluado dentro del lock |
| RC3. TOCTOU city cache | ✅ Resuelto | Double-check locking implementado |
| P1. Crypto secuencial | ✅ Resuelto | BTC/ETH paralelos con `sync.WaitGroup` |
| P2. Sports O(n*m) | ✅ Resuelto | Map `promiedosLookup` para O(1) |
| R2. String concat en loop | ✅ Resuelto | `strings.Builder` en alerts |
| Q1. Código duplicado DaysAbbr | ✅ Resuelto | Centralizado en `common.DaysAbbr` |
| Q1. Código duplicado normalizeName | ✅ Resuelto | Centralizado en `common.NormalizeName` |
| Q5. Puerto hardcodeado | ✅ Resuelto | `PORT` env var |
| Q7. test_alerts.go en root | ✅ Resuelto | Eliminado |
| M1. Sin timeouts HTTP | ✅ Resuelto | `ReadTimeout`, `WriteTimeout`, `IdleTimeout` |
| M2. Sin graceful shutdown | ✅ Resuelto | Signal handling con `context.WithTimeout` |
| D1. Container como root | ✅ Resuelto | `appuser:appgroup` + `USER appuser` |
| D2. alpine:latest unpinned | ✅ Resuelto | `alpine:3.20` |
| D3. Sin HEALTHCHECK | ✅ Resuelto | `wget --spider` cada 30s |
| D8. .dockerignore incompleto | ✅ Resuelto | Agregados `conductor/`, `Dockerfile`, `docker-compose.yml`, etc. |
| Q4. Holidays hardcoded | ✅ Parcial | Fechas fijas son dinámicas por año, variables solo para 2026 |

---

## Issues restantes

### 1. Seguridad

#### S-A. SSRF por IP directa — `internal/network/network.go:13-32`
La allowlist solo valida dominios, no IPs. Se puede bypassar resolviendo `upload.wikimedia.org` a una IP interna con DNS malicioso, o accediendo directamente a `http://169.254.169.254/` (cloud metadata).

**Fix:** Agregar validación de IP en `FetchSecure`:
```go
// Después de resolver DNS, verificar que la IP no sea privada
ips, _ := net.LookupIP(hostname)
for _, ip := range ips {
    if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() {
        return nil, fmt.Errorf("IP interna bloqueada")
    }
}
```

#### S-B. Sin política de redirect en HTTP client — `internal/network/network.go:35-42`
`DefaultClient` sigue redirects automáticamente. Un dominio allowlisted podría redirigir a `http://169.254.169.254/`.

**Fix:**
```go
DefaultClient = &http.Client{
    Timeout: 15 * time.Second,
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        if len(via) >= 3 {
            return fmt.Errorf("demasiados redirects")
        }
        // Re-validar dominio del redirect contra allowlist
        allowed, _ := IsDomainAllowed(req.URL.String())
        if !allowed {
            return fmt.Errorf("redirect a dominio no permitido")
        }
        return nil
    },
    // ...
}
```

#### S-C. Dominio `promiedos.com.ar` permite SSRF — `internal/sports/sports.go:508`
`fetchPromiedosChannels` usa `http.Client` directo (no `FetchSecure`) para acceder a `promiedos.com.ar`. Esto bypassa la allowlist. El contenido scrapeado del HTML se parsea como JSON. Si Promiedos es comprometido o devuelve contenido inesperado, puede causar panics.

**Riesgo:** Bajo (dominio confiable), pero inconsistente con el patrón de seguridad.

---

### 2. Bugs

#### B-A. CoinGecko success silencioso con datos inválidos — `internal/crypto/crypto.go:110`
```go
if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result[cgID].USD > 0 {
    return result[cgID].USD, result[cgID].USDChange, nil
}
```
Si CoinGecko devuelve 200 con JSON válido pero sin la key `cgID` (cambio de API), `result[cgID].USD` es 0, el código cae al fallback Binance correctamente. OK, pero sin log del fallback.

#### B-F. Error variable shadowing en Binance fallback — `internal/crypto/crypto.go:121`
```go
if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
```
La variable `err` del scope externo es reescrita por `:=` del scope interno. Si el decode falla, se pierde el error original. No es un bug crítico pero dificulta debugging.

#### B-G. `fmt.Printf` en vez de `log` — `internal/sports/sports.go:69`
```go
fmt.Printf("[SPORTS] Update completado en %v. Partidos: %d\n", ...)
```
`fmt.Printf` no agrega timestamp. Debería ser `log.Printf`.

#### B-H. `fmt.Sscanf` sin verificar error — `cmd/main.go:337-338`
```go
fmt.Sscanf(lat, "%f", &fLat)
fmt.Sscanf(lon, "%f", &fLon)
```
Si `lat` o `lon` no son números válidos, `fLat`/`fLon` quedan en 0, y `0 >= -90` es true. Las coordenadas (0,0) son un punto real en el Golfo de Guinea. No es un bug de seguridad pero sí un edge case.

**Fix:** Verificar error de `Sscanf`:
```go
if _, err := fmt.Sscanf(lat, "%f", &fLat); err != nil {
    return // o rechazar
}
```

---

### 3. Optimizaciones de rendimiento

#### P-A. Weather cache eviction destruye todo — `internal/weather/weather.go:125-127`
```go
if len(weatherCache) > maxCacheEntries {
    weatherCache = make(map[string]*cachedWeatherItem)
}
```
Cuando se superan 100 entradas, se eliminan TODAS las entradas. Esto causa thundering herd: todas las ciudades se refetchean simultáneamente.

**Fix:** Eliminar la entrada más antigua (LRU simple):
```go
if len(weatherCache) >= maxCacheEntries {
    var oldestKey string
    var oldestTime time.Time
    for k, v := range weatherCache {
        if oldestKey == "" || v.Timestamp.Before(oldestTime) {
            oldestKey = k
            oldestTime = v.Timestamp
        }
    }
    delete(weatherCache, oldestKey)
}
```

#### P-B. Misma política en city cache — `internal/weather/weather.go:259-261`
Mismo problema: destrucción total del cache de ciudades.

#### P-C. Llamadas Binance secuenciales — `internal/crypto/crypto.go:117-133`
Cuando CoinGecko falla, las dos llamadas a Binance (precio + stats 24h) son independientes pero se hacen secuenciales.

**Fix:** Paralelizar con WaitGroup.

#### P-D. Llamadas Dólar secuenciales — `internal/finance/finance.go:84-101`
Blue y Cripto se piden secuencialmente. Son independientes.

**Fix:** Paralelizar.

#### P-E. SPY/QQQ secuenciales — `internal/finance/finance.go:104-109`
Dos llamadas a Yahoo Finance independientes, secuenciales.

**Fix:** Paralelizar con las 4 fuentes financieras juntas.

#### P-F. Earthquake deduplicación O(n²) — `internal/earthquake/earthquake.go:161-181`
Con máximo 20 elementos (10 USGS + ~10 INPRES), el impacto es mínimo. No urgente.

#### P-G. GetCrestURL triple iteración — `internal/sports/sports.go:209-233`
1. Casos específicos (hardcoded returns)
2. Búsqueda exacta en map
3. Búsqueda parcial en map

La búsqueda parcial (paso 3) itera todo el map. Con ~30 entradas es imperceptible. No urgente.

---

### 4. Refactorizaciones

#### R-A. Feriados variables solo para 2026 — `internal/holidays/holidays.go:43-58`
El bloque de feriados trasladables tiene un `if year == 2026`. Después del 31/12/2026, solo se muestran los inamovibles. Hay un TODO para integrar una API.

**Opciones:**
1. Integrar API de `nolaborables.com.ar` o similar
2. Hardcodear los feriados para cada año futuro conocido
3. Calcular fechas móviles (Carnaval, Pascuas) con algoritmo

#### R-B. `strings.ReplaceAll` encadenado en NormalizeName — `internal/common/common.go:19-34`
8 llamadas a `strings.ReplaceAll`. Podría ser un `strings.Map` o `regexp` más limpio.

```go
func NormalizeName(name string) string {
    replacer := strings.NewReplacer(
        "á","a","é","e","í","i","ó","o","ú","u",
        "'","","'",""," ","","(", "",")","",
        ".","","-","",",","",
    )
    return replacer.Replace(strings.ToLower(name))
}
```

#### R-C. Struct inline masivo en sports — `internal/sports/sports.go:368`
La línea 368 tiene ~1200 caracteres de struct JSON inline para la respuesta F1 de Ergast. Difícil de leer y mantener.

**Fix:** Extraer a type definitions separados.

#### R-D. `getMoonPhaseInfo` devuelve siempre "moon" — `internal/weather/weather.go:196-212`
Todas las fases retornan el mismo icono `"moon"` excepto "Llena" que retorna `"circle"`. Si la UI de Lucide soporta fases lunares específicas, usar iconos distintos por fase (si están disponibles).

#### R-E. `fmt.Sprintf` para concatenar strings — `internal/crypto/crypto.go:105`
```go
urlCG := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd&include_24hr_change=true", cgID)
```
Podría usar `url.Values` o concatenación simple para URLs. Menor prioridad.

#### R-F. `fetchLiveMatches` y `fetchLiveUFC` usan `http.Client` directo — `internal/sports/sports.go:289-290,428`
Crean su propio `http.Client` con timeout en vez de usar `network.DefaultClient`. Esto duplica configuración y no respeta los ajustes del cliente global (conexiones reutilizables, TLS handshake timeout, etc.).

**Fix:** Usar `network.DefaultClient` o crear un helper `network.FetchSecureWithTimeout`.

---

### 5. Docker

#### D-A. Puerto expuesto a toda la red — `docker-compose.yml:8`
`"8060:8060"` bindea a `0.0.0.0`. Para un dashboard personal:
```yaml
ports:
  - "127.0.0.1:8060:8060"
```

#### D-B. Sin límites de recursos
```yaml
deploy:
  resources:
    limits:
      memory: 128M
      cpus: '0.5'
```

#### D-C. Sin `cap_drop`
```yaml
cap_drop:
  - ALL
```

#### D-Build. Build lento en cambio de versión Go
El cambio de imagen base invalida todas las capas. No es un problema del Dockerfile sino inherente a Docker. Los builds subsiguientes con cache serán rápidos.

---

## Resumen por Severidad

| Severidad | Cantidad | Issues |
|-----------|----------|--------|
| **Medio** | 8 | SSRF por IP (S-A), redirect policy (S-B), cache eviction (P-A, P-B), Binance secuencial (P-C), dólar secuencial (P-D, P-E), feriados 2026 (R-A) |
| **Bajo** | 8 | Shadowing error (B-F), fmt.Printf (B-G), Sscanf sin check (B-H), NormalizeName refactor (R-B), struct inline (R-C), moon icon (R-D), sports HTTP client (R-F), docker puerto (D-A) |
| **Info** | 3 | Recursos Docker (D-B), cap_drop (D-C), Promiedos SSRF (S-C) |

**Total: 19 issues restantes** (de 36 originales, 17 resueltos)

---

## Prioridad de implementación

1. **Corto plazo:** SSRF por IP (S-A), redirect policy (S-B), cache eviction (P-A, P-B)
2. **Medio plazo:** Paralelizar Binance/finanzas (P-C, P-D, P-E), feriados dinámicos (R-A), sports HTTP client (R-F)
3. **Largo plazo:** Refactorizaciones de código (R-B, R-C, R-D), Docker hardening (D-A, D-B, D-C)
