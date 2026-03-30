# Homedash - Homelab Dashboard V1.3 (Auditoría y Refactorización)

Dashboard minimalista optimizado para carga instantánea, personalización individual y visualización de datos en vivo.

## Stack Tecnológico y Arquitectura (V1.3 Updates)
- **Backend:** Go (Golang) con caché atómica, protección SSRF y concurrencia optimizada.
- **Seguridad:** Implementación de `internal/network` para control de dominios permitidos (Whitelist).
- **Frontend:** HTMX para recargas parciales cada 60s, TailwindCSS (DaisyUI).
- **Despliegue:** Docker con usuario no privilegiado (`appuser`) y healthchecks.

## Registro de Cambios y Características (Contexto)
... (puntos 1-20 se mantienen como referencia histórica) ...

21. **Refactorización de Seguridad (Whitelist SSRF):**
    - Se creó el paquete `internal/network` que centraliza todas las peticiones HTTP externas.
    - Implementación de una lista blanca de dominios permitidos para el proxy de escudos, mitigando ataques de SSRF.
22. **Prevención de Panics y Errores:**
    - Corrección sistémica de "Nil Dereference" en llamadas HTTP (verificando errores antes de acceder a `resp.StatusCode`).
    - Cierre garantizado de `resp.Body` para evitar fugas de descriptores de archivos.
23. **Seguridad Web:**
    - Escape de HTML en todos los campos de autocompletado (Ciudades/Equipos) para prevenir inyecciones XSS.
    - Cookies de configuración actualizadas con flags `Secure` y `SameSite: Lax`.
24. **Optimización de Caché y Concurrencia:**
    - Implementación de *Double-Check Locking* en las cachés de Clima y Ciudades para evitar el fenómeno de *thundering herd*.
    - Limite de tamaño en cachés (`maxCacheEntries`) para prevenir el consumo infinito de memoria.
    - Paralelización de peticiones HTTP independientes (BTC+ETH, Clima+AQI, Múltiples ligas de ESPN) usando `sync.WaitGroup`, reduciendo significativamente el tiempo de respuesta.
25. **Arquitectura Limpia:**
    - Creación del paquete `internal/common` para unificar lógica de normalización de nombres y constantes de fechas.
    - Unificación de funciones de normalización de provincias y equipos.
    - Eliminación de código muerto, funciones `init` vacías y librerías obsoletas (`strings.Title`).
26. **Robustez del Servidor:**
    - Configuración de `http.Server` con timeouts (`Read/Write/Idle`).
    - Implementación de `Graceful Shutdown` para cerrar el servidor limpiamente ante señales del sistema.
27. **Mejoras en Docker:**
    - El contenedor ya no corre como `root`; ahora utiliza un usuario no privilegiado `appuser`.
    - Base de imagen fijada a `alpine:3.20` para builds reproducibles.
    - Añadido `HEALTHCHECK` oficial de Docker para monitoreo de salud.
28. **Mejoras UI F1 (Mobile):**
    - Se eliminó el límite de altura (`h-full`) y `overflow-hidden` en mobile para permitir que la tarjeta de deportes se expanda según el contenido.
    - Reubicación de los iconos de clima en F1 para evitar solapamientos con el nombre de la sesión.
    - Implementación de `past_days=2` en la API de Clima para recuperar iconos de sesiones pasadas durante el fin de semana de carrera.

29. **Mejoras UI y Datos de Fútbol:**
    - Se redujo el tamaño de fuente y se añadió padding a la derecha al nombre del Gran Premio de F1 en mobile para evitar que se corte.
    - Se agregaron la Copa Argentina y la Copa Libertadores a las fuentes de datos de fútbol para asegurar la visibilidad de todos los torneos relevantes.
    - Se ajustó la lógica de prioridad de partidos: ahora los partidos que se juegan HOY o están EN VIVO siempre tienen prioridad sobre partidos futuros, independientemente del torneo.
30. **Homedash V2.0 - Refactorización y Hardening:**
    - **Seguridad:** Implementación de middleware para security headers (CSP, HSTS, XSS), bloqueo de IPs internas en `FetchSecure` (SSRF Hardening) y validación estricta de coordenadas.
    - **Modularización:** División del paquete `internal/sports` en archivos específicos (football, f1, ufc, promiedos) para mejorar la mantenibilidad.
    - **Estabilidad:** Adición de contexts con timeout en todos los bucles de actualización y uso de `LimitReader` para evitar OOM al procesar HTML externo.
    - **UI:** Creación de un layout base (`layout.html`) para evitar duplicación de boilerplate en plantillas y mejora de iconos de fase lunar.
    31. **Actualización de Finanzas, UFC y F1 Mobile:**
    - **UFC:** Se ajustó la lógica de selección de eventos para omitir eventos finalizados y priorizar el próximo evento programado (o en vivo).
    - **F1 UI:** Restauración de fechas en sesiones y fecha del Gran Premio tras optimización de espacio.
    - **Sismos:** Restauración del contador de sismos a 5 eventos para mantener la visibilidad histórica.
    - **F1 Mobile:** Se eliminó la clase `hidden` en el trazado del circuito (PosterURL) para dispositivos móviles.
    - **F1 Sprint:** Soporte para `SprintQualifying` (SQualy).
    - **Finanzas:** Seguimiento del **Riesgo País** (ArgentinaDatos) con porcentaje de cambio y colores dinámicos.
    - **UI:** Optimización de espacio vertical preservando legibilidad y restaurando indicadores de viento/lluvia.    - **UI:** Se reemplazaron los CEDEARs de SPY/QQQ por los índices globales S&P 500 (^GSPC) y NASDAQ (^IXIC) en dólares.
    - Se habilitó la selección de texto en la tarjeta de UFC para facilitar el copiado del nombre del evento.
    - Se centró la cuadrícula de sesiones de F1 en modo escritorio para un mejor balance visual cuando hay menos de 6 eventos.
    - Se añadió el prefijo "U$S " a los activos financieros en dólares (BTC, ETH, SP 500, NASDAQ) para distinguirlos de los precios en pesos.
    - Se añadieron las fotos (headshots) de los peleadores del evento principal de UFC, obtenidas dinámicamente de la API de ESPN.
    ## Arquitectura Dinámica y Fuentes de Datos
32. **Mejoras Visuales y Favicon:**
    - Se forzó el diseño de una sola fila para las sesiones de F1 en escritorio (`flex-nowrap`) y se centraron los elementos.
    - Se implementó la visualización de la bandera del país de la carrera en F1, con mapeo automático de países a iconos locales y se restauró el trazado del circuito.
    - Se descargó el favicon localmente para resolver problemas de visualización por políticas de seguridad (CSP).
    - Se añadió un espacio entre el símbolo `$` y el valor numérico en los activos financieros en pesos (Dólar Blue/Cripto) para mayor legibilidad.
33. **Proxy Manager & Seguridad (Auditoría v2):**
    - Se implementó validación de `X-Forwarded-For` y `X-Forwarded-Proto` confiando solo en proxies de IPs privadas (Loopback, LAN).
    - Se implementó middleware manual de compresión `gzip` y middleware de `Cache-Control` (24h) para recursos estáticos (`/static/`), mejorando el rendimiento.
    - Se limpió el diseño del paquete `weather` moviendo `WeatherViewModel` a `cmd/main.go` para eliminar dependencias circulares y mejorar la arquitectura.
    - Se añadieron etiquetas OCI (Labels) al `Dockerfile` y se actualizaron las exclusiones de `.dockerignore` para optimizar el tamaño y metadatos de la imagen.
34. **Optimización Extrema de Docker (Performance & Security):**
    - **Arquitectura:** Migración de imagen base `alpine` a `gcr.io/distroless/static-debian12:nonroot` (Hardening extremo).
    - **Build Time:** Reducción del tiempo de construcción consolidando assets y binario en una única capa de `COPY` con permisos (`--chown`) nativos.
    - **Caché:** Reordenamiento estratégico: assets se copian *después* del `go build` para proteger la caché de compilación de cambios en HTML/estáticos.
    - **Robustez:** Inyección manual de `tzdata` desde el builder para garantizar precisión horaria en entornos distroless.
35. **Remediación Masiva de Bugs (Fase 2):**
    - **Estabilidad Backend:** Fix de *Nil Pointer Panic* en `weather.go` forzando la inicialización del puntero antes del Decode. Implementación de *Double-Check Locking* en caché de ciudades. Propagación de `context.Context` hacia APIs de clima y sismos. Sincronización con `sync.Mutex` en `fetchFreshSportsData` eliminando el data race.
    - **Seguridad y Control de Errores:** Validación de Content-Type en `handleCrestProxy` (MIME sniffing prevention) y extensión `.svg` en minúsculas. Manejo explícito de `io.ReadAll` y errores de plantilla. Purgado selectivo del `rateLimitMap` (evita que un ataque resetee a todos los usuarios válidos).
    - **Frontend (A11y, Performance, CSP):** Atributos `alt` descriptivos y `loading="lazy"` en todas las imágenes/escudos. Soporte de semántica `aria` y vinculación de `for/id` en el modal. Eliminados eventos inline (`onclick`, `onerror`) manejados vía scripts. Soporte de `prefers-reduced-motion` y fallback global de `<noscript>`. Fuentes de Google pasadas a `<link>` (no-blocking).
- **Fútbol:** `Promiedos` (vía JSON scraping) + `ESPN` (paralelizado).
- **UFC:** API oficial de `ESPN` filtrada por fechas.
- **F1:** API de `Ergast` + Pronóstico local por sesión.
- **Finanzas:** Yahoo Finance (CEDEARs), Binance/CoinGecko (Cripto), DolarAPI (Dólar).
- **Sismos:** USGS + INPRES (Combinados y deduplicados en paralelo).

## Convenciones de Desarrollo (OBLIGATORIO)
- **Idioma:** Todos los mensajes de commit, comentarios relevantes y documentación interna deben redactarse en **español de Argentina** (usando voseo y terminología técnica local).
- **Integridad de Datos:** **PROHIBIDO** eliminar o remover cualquier dato o indicador visual existente sin el consentimiento expreso del usuario. Cualquier optimización de espacio debe priorizar la reubicación o el redimensionamiento sobre la eliminación.
- **Documentación de Contexto:** Al final de cada sesión, actualizar `GEMINI.md`.
- **Validación:** Ejecutar `go build ./...` y verificar rigurosamente la sintaxis de todos los archivos modificados (Go, HTML Templates, Shell scripts, etc.) antes de cualquier sincronización.
- **Sincronización:** `rsync -avz --exclude '.git' --exclude 'homedash' -e "ssh -p 4370" . kastor@cronix.com.ar:/home/samba/docker/homedash`.
- **Red:** Usar `network.FetchSecure` para cualquier petición externa.
- **Seguridad:** Nunca confiar en entradas del usuario (`r.FormValue`) sin validar rangos o tipos.
