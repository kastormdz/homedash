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
36. **Mundial 2026 Fixture:**
    - Implementación de un fixture completo para el Mundial 2026 (ESPN API).
    - Nuevo modal en la tarjeta de fútbol que carga los partidos mediante HTMX al hacer clic en "FIXTURE Mundial".
    - Agrupación de partidos por etapas (Fase de grupos, Dieciseisavos, etc.) con banderas dinámicas.
37. **Correcciones de UI, Feriados y Mundial:**
    - **Feriados:** Implementación de ordenamiento cronológico (`sort.Slice`) y resaltado visual "pro" (`animate-pulse` + `sparkles`) para feriados en los próximos 2 días mediante el nuevo flag `IsNear`.
    - **Mundial 2026:** Traducción completa del Fixture al español: cambio de "Bracket" por "Llaves" y mapeo de etapas (ej. "Round of 16" a "Octavos de final") en el backend.
    - **UI:** Cambio de color del botón "FIXTURE Mundial" a `amber-400` para mejor contraste estético.

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
- **AI-Ready (OBLIGATORIO):** Toda nueva funcionalidad, fuente de datos o indicador visual debe ser expuesto obligatoriamente en la API JSON (`/api/v1/*`) y en el servidor MCP (`tools/list` y `handleToolCall`) para que los agentes de IA locales puedan consumirlos.
- **Sincronización y Deploy:**
  1. **Sync:** `rsync -avz --exclude '.git' --exclude 'homedash' -e "ssh -p 4370" . kastor@cronix.com.ar:/home/samba/docker/homedash`
  2. **Rebuild:** `ssh -p 4370 kastor@cronix.com.ar "cd /home/samba/docker/homedash && docker compose up --build -d"`
- **Red:** Usar `network.FetchSecure` para cualquier petición externa.
- **Seguridad:** Nunca confiar en entradas del usuario (`r.FormValue`) sin validar rangos o tipos.

38. **Integración del Tema "Nothing Style":**
    - Se agregó soporte completo para un nuevo tema inspirado en Nothing Phone, configurado a través del script de Tailwind en `layout.html` (mapeo de primary, neutral y radios de bordes).
    - Implementación de `.nothing-dot-bg` (radial-gradient) para el fondo de pantalla y la fuente `DotGothic16` para números y encabezados.
    - Se aplicaron ajustes UI dinámicos (padding en tarjetas, iconos grises/zinc) usando selectores CSS `[data-theme="nothing"]` para evitar acoplamiento fuerte en las plantillas HTML.
39. **Efecto Confetti y Highlight para Feriados:**
    - Se reemplazó la animación `animate-pulse` en feriados por un efecto de confetti cayendo (`@keyframes confetti-fall`) implementado en CSS puro dentro de la tarjeta del reloj.
    - Activación dinámica de confetti exclusivamente el día del feriado mediante la variable `.Holiday`, manteniendo el rendimiento sin JS.
    - Se amplió el umbral de feriados próximos (IsNear) de 2 a 5 días y se agregó un sutil efecto de resaltado (`holiday-item-near`) usando `@keyframes subtle-pulse` en el fondo.
40. **Corrección de Datos de Fútbol (Libertadores, Sudamericana, Copa Argentina):**
    - Se actualizaron los slugs de la API de ESPN para corregir la obtención de partidos de CONMEBOL Libertadores y Sudamericana.
    - Se añadió el slug `arg.copa_lpf` para asegurar la visibilidad de la Copa de la Liga Profesional.
    - Rediseño del algoritmo de scoring en `GetSportsDataForUser` para priorizar partidos por proximidad temporal real (usando `RawDate`) además de la importancia del torneo.
    - Los partidos de hoy o en vivo mantienen la prioridad absoluta sobre cualquier partido futuro.
41. **Actualización en Tiempo Real pre-partido:**
    - Se modificó el bucle de actualización de deportes (`StartUpdateLoop`) para reducir el intervalo a **1 minuto** cuando falten menos de 30 minutos para el inicio de cualquier partido.
    - Esto asegura que los cambios en el estado (EN VIVO), alineaciones y los primeros minutos de juego se reflejen casi instantáneamente en el dashboard.
42. **Optimización de Modal y Corrección de Layout:**
    - Se eliminó una llamada redundante a la API de geocoding en `getSettings`, que causaba lentitud (bloqueo por red) cada vez que se cargaban las preferencias sin una provincia en la cookie.
    - Se reemplazó la recarga total de la página (`HX-Refresh`) por un trigger específico (`refreshWeather`) que actualiza solo los componentes de clima/deportes inmediatamente al guardar cambios, mejorando drásticamente la velocidad de respuesta percibida.
    - Se corrigió un bug en el sistema de grillas (`lg:col-span`) de la fila inferior donde la suma de columnas excedía 12 al ocultar la tarjeta de UFC, lo que provocaba que la tarjeta de Finanzas se desplazara hacia abajo. Ahora las grillas se adaptan dinámicamente (6+6, 5+3+4, etc.) para mantener la alineación horizontal.
43. **Refactorización de Rendimiento (Caché Asíncrona para Sismos y Alertas):**
    - Se solucionó el problema crítico de llamadas HTTP bloqueantes en `/weather` (B-1).
    - Se implementó un bucle en segundo plano (`StartUpdateLoop` y `StartAlertsLoop`) con variables de paquete protegidas por Mutex en `internal/earthquake/earthquake.go` (P-1) y `internal/weather/alerts.go` (P-2).
    - Ahora el manejador lee los datos instantáneamente de la memoria caché. Esto reduce el tiempo de renderizado de la tarjeta del clima a milisegundos y previene que el dashboard se caiga si las APIs del SMN o INPRES fallan o están lentas.
44. **Refactorización de Fase 1 (B-3, B-6) y Fase 2 (P-4, P-5, P-6, B-2):**
    - **B-3**: Se añadieron logs de error en `handleCrestProxy` al guardar en caché local.
    - **B-6**: Se implementó un bucle de limpieza con `time.Ticker` para el mapa de rate-limiting previniendo memory leaks.
    - **P-4**: Se pre-computaron los nombres normalizados de equipos y torneos en `fetchFreshSportsData`, eliminando operaciones costosas de strings de orden O(n) durante cada request de usuario.
    - **P-5**: Se cacheó la predicción del clima del circuito de F1 por 1 hora para evitar golpear la API de Open-Meteo innecesariamente.
    - **P-6**: Se paralelizó la obtención del Riesgo País mediante un *Race Pattern* (Ámbito vs ArgentinaDatos), reduciendo el tiempo de espera.
    - **B-2**: Se corrigió el Race Condition en las tendencias de BTC y ETH (Trend) unificándolas bajo los bloqueos del paquete de `crypto`.
45. **Refactorización de Fase 3 (Seguridad e Infraestructura):**
    - **B-5**: Se implementó un `DialContext` personalizado en `network.DefaultClient` que resuelve IPs y bloquea destinos internos *antes* de conectar, mitigando ataques de DNS Rebinding (TOCTOU) durante el proxyado HTTP.
    - **B-8**: Se eliminó la directiva `'unsafe-inline'` del header Content-Security-Policy (CSP) aislando todos los scripts de inicialización y de UI en un nuevo archivo externo `static/app.js`, reduciendo dramáticamente la superficie de ataque para inyecciones XSS.
    - **A-5**: Se actualizaron los límites de recursos de Docker en `docker-compose.yml` (`mem_limit: 128M` y `cpus: 0.5`), pasando del esquema ignorado de Swarm (`deploy.resources`) a la sintaxis V2/V3 para asegurar que el contenedor está estrictamente restringido en el host.
46. **Refactorización de Fase 4 (Estabilidad de Plantillas):**
    - **B-11, P-7**: Se estandarizó el uso de `bytes.Buffer` antes de escribir al `ResponseWriter` en todas las plantillas (especialmente en WorldCup y F1), asegurando que si ocurre un error a mitad del renderizado, el cliente no reciba un HTML corrupto.
47. **Remediación de Iconos y Refactorización Frontend:**
    - Se corrigió el orden de carga de los scripts en `layout.html`, moviendo Lucide y Tailwind antes de `app.js` para evitar errores de referencia (Race Condition).
    - Se refactorizó `static/app.js` envolviendo la inicialización en un evento `DOMContentLoaded`, garantizando que el DOM esté disponible antes de ejecutar `lucide.createIcons()` y `updateClock()`.
    - Se migró el listener de HTMX (`htmx:afterSwap`) de `document.body` a `document` para mayor robustez ante cargas asíncronas de componentes.
    - Limpieza de código redundante y corrección de etiquetas `</div>` duplicadas en la plantilla base.
48. **Ruta de Test y Refinamiento Nothing Style:**
    - Creación de la ruta `/test` para previsualizar cambios de diseño sin afectar la sesión del usuario.
    - Rediseño experto del tema 'nothing': implementación de dot matrix background, glassmorphism avanzado (blur 16px), tipografía DotGothic16 para datos y acentos en rojo vibrante (#ff0000).
    - Cumplimiento estricto de las reglas de integridad HTML (sin cambios en IDs ni estructura).
49. **Optimización Extrema de Espacio (Anti-Scroll):**
    - Reducción general de paddings del body (0.15rem) y dashboard container (0.1rem).
    - Compactación de tarjetas: reducción de padding interno (0.85rem) y altura mínima del reloj (180px).
    - Ajuste de componentes de clima: reducción de tamaño de iconos y paddings en alertas/sismos.
    - Optimización de deportes: márgenes reducidos entre bloques de fútbol y F1.
    - Footer ultra-compacto para garantizar visibilidad del link de configuración sin scrolling.

50. **Homedash API & MCP (AI Integration):**
    - **API JSON v1:** Implementación de endpoints `/api/v1/*` que exponen los datos de las cachés internas en formato JSON para consumo programático.
    - **Servidor MCP Nativo:** Integración del Model Context Protocol (MCP) sobre `stdio`. Ahora Homedash puede actuar como un servidor de herramientas para agentes locales (como Claude Desktop) mediante el comando `./homedash mcp`.
    - **Modularización:** Refactorización de la lógica de API y MCP en archivos separados (`cmd/api.go` y `cmd/mcp.go`) para mantener `main.go` limpio.
    - **Herramientas para IA:** Exposición de herramientas semánticas (`obtener_clima`, `obtener_deportes`, `obtener_finanzas`, etc.) con esquemas JSON validados.
    - **Robustez:** Las herramientas MCP utilizan las mismas cachés atómicas y bucles de actualización que el dashboard visual, garantizando datos frescos sin carga extra de red.

51. **Estabilidad de Red y Fallback de Clima (Anti-429):**
    - **Robustez de Red:** Refactorización de internal/network/network.go usando net.Dialer.Control para una protección SSRF más limpia y compatible con TLS/SNI. Se aumentó el timeout global a 30s para absorber retardos de APIs externas.
    - **Fallback de Datos Obsoletos:** Implementación de Stale Data Fallback en el paquete weather. Si la API de Open-Meteo falla (ej. error 429), el sistema devuelve los últimos datos conocidos de la caché en lugar de un error.
    - **Período de Enfriamiento:** Se añadió un mecanismo de 'enfriamiento' (cooldown) de 2 minutos tras un fallo de API, evitando saturar a los proveedores durante bloqueos por rate-limit.
    - **Feedback UI:** Inclusión del flag IsAvailable en el ViewModel de clima, permitiendo mostrar un estado visual claro ('Clima no disponible') en lugar de datos erróneos o en cero cuando no existe ni siquiera información en caché.

52. **Remediación de Riesgo País y Hardening de Ámbito:**
    - **Finanzas:** Se corrigió el problema de actualización del Riesgo País mediante el uso de `network.FetchSecureWithContext` para la fuente de Ámbito, asegurando que se envíen los headers necesarios (User-Agent, Accept) para evitar bloqueos de seguridad (403 Forbidden).
    - **Estabilidad:** Implementación de logs de error específicos (`[FINANCE]`) para cada fuente de Riesgo País (Ámbito y ArgentinaDatos) en el patrón de race concurrency, facilitando la depuración de fallos individuales.
    - **UI:** Corrección de la lógica de colores del Riesgo País: ahora se muestra en **rojo** (error) cuando sube y en **verde** (éxito) cuando baja, alineándose con la semántica financiera correcta.
    - **Robustez:** Mejora del patrón de "Stale Data Fallback" asegurando que el dashboard siempre muestre el último valor conocido si ambas fuentes fallan temporalmente.

53. **Rediseño Industrial "Nothing Style" (v2.0 Extreme):**
    - **Tipografía:** Implementación de `Space Grotesk` (Primary/Secondary) y `Space Mono` (Tertiary/Metadata), manteniendo `DotGothic16` para el reloj y momentos "hero".
    - **Visual Hardening:** Eliminación de sombras y blurs en el tema `nothing`, priorizando fondos OLED negro absoluto (`#000000`) y bordes industriales de 1px.
    - **Modales de Ingeniería:** Rediseño completo de los modales de Ajustes, Mundial y F1 con estética monocromática, botones invertidos (White/Black) y labels en monospace ALL CAPS.
    - **Monocromía:** Aplicación de filtros `grayscale` y `brightness` a logos, escudos y banderas para una estética técnica, reservando el rojo `#D71921` exclusivamente para alertas y acentos de estado.
    - **Jerarquía de 3 Capas:** Implementación estricta de la regla de jerarquía (Display, Body, Metadata) basada en peso tipográfico y espaciado de 8px, eliminando divisores innecesarios.
    - Dot Matrix: Expansión del patrón de puntos (`dot-matrix-bg`) a encabezados de modales y contenedor del reloj para acentuar el ADN de Nothing.

    54. **Mejoras en Cartelera de UFC:**
    - **Estructura de Datos:** Migración de `MainCard []string` a `Fights []UFCFight`, permitiendo el manejo estructurado de competidores, ganadores y estados.
    - **Highlight de Ganadores:** Implementación de resaltado visual (`text-error`) en el nombre del peleador victorioso mediante la detección del campo `winner` de la API de ESPN.
    - **Estado de Pelea:** Visualización dinámica de etiquetas `FINAL` y `EN VIVO` para cada pelea individual en la cartelera.
    - **Expansión de Contenido:** Incremento del límite de peleas mostradas de 4 a 5 filas, optimizando el interlineado (`space-y-1.5`) para mantener el diseño anti-scroll.

## graphify

This project has a graphify knowledge graph at graphify-out/.

Rules:
- Before answering architecture or codebase questions, read graphify-out/GRAPH_REPORT.md for god nodes and community structure
- If graphify-out/wiki/index.md exists, navigate it instead of reading raw files
- For cross-module "how does X relate to Y" questions, prefer `graphify query "<question>"`, `graphify path "<A>" "<B>"`, or `graphify explain "<concept>"` over grep — these traverse the graph's EXTRACTED + INFERRED edges instead of scanning files
- After modifying code files in this session, run `graphify update .` to keep the graph current (AST-only, no API cost)
