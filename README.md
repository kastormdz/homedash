# Homedash

Homedash es un dashboard minimalista de informacion util/inutil , optimizado para carga instantánea, personalización individual y visualización de datos en tiempo real. Está diseñado con la filosofía de **"Cero Hardcoding"**, alimentándose completamente de múltiples APIs para mantenerse siempre actualizado sin requerir intervención manual.

## Arquitectura y Stack Tecnológico

- **Backend:** Go (Golang) con concurrencia nativa (Goroutines) para mantener una caché en memoria ultra-rápida.
- **Frontend:** HTMX para recargas parciales y silenciosas (cada 60s), logrando una experiencia de Single Page Application (SPA) sin escribir JavaScript complejo.
- **Estilos y UI:** Tailwind CSS integrado con **DaisyUI** para soportar temas dinámicos (Dark, Dracula, Synthwave, etc.).
- **Iconografía:** Lucide Icons.
- **Persistencia:** Sistema de preferencias basado en Cookies codificadas en JSON, permitiendo a múltiples usuarios en la misma red tener configuraciones completamente distintas sin necesidad de base de datos.
- **Infraestructura:** Contenerizado con Docker (Build multi-stage en Alpine) resultando en una imagen final ultra-ligera (~30MB).

---

## Funcionalidades Principales

### Deportes en Vivo
- **Fútbol Argentino e Internacional:** Extrae datos en tiempo real de Promiedos mediante web scraping de su objeto JSON. Muestra resultados en vivo, minutero, y detecta automáticamente en qué canal (TNT Sports, ESPN Premium, etc.) lo transmiten. Incluye un sistema de desambiguación para equipos con nombres similares (ej. Racing Club vs Racing de Córdoba).
- **Fórmula 1:** Utiliza la API de Ergast para mostrar la próxima carrera, el circuito, y los horarios locales de todas las sesiones (FP1, FP2, Qualy, Sprint, Race). **¡Novedad!** Calcula geográficamente dónde es el circuito e inyecta el **pronóstico del clima exacto** para los días de cada sesión.
- **UFC:** Integrado con la API oficial de ESPN. Muestra el estado del evento (previa, en vivo, finalizado) y lista las peleas del Main Card. Consulta inteligentemente un rango de 30 días para asegurar que, una vez finalizado el evento del sábado, cambie automáticamente a la cartelera del próximo.

### Clima y Alertas
- **Pronóstico Semanal y Actual:** Datos proporcionados por Open-Meteo. Muestra temperatura, probabilidad de lluvia, humedad, índice UV y un pronóstico a 7 días.
- **Calidad del Aire (AQI):** Indicador visual en tiempo real. Si el aire es perjudicial, se resalta con alertas pulsantes.
- **Alertas Meteorológicas (SMN):** Para usuarios de Argentina, se conecta al feed oficial del Servicio Meteorológico Nacional (CAP/RSS). Detecta tu provincia y genera un **resumen inteligente y corto** (ej: "Viento 50 km/h" con icono) de las alertas vigentes en tu región.
- **Fase Lunar:** Un algoritmo astronómico interno calcula en Go la fase exacta de la luna (sin depender de APIs externas) y la muestra con su icono correspondiente.

### Sismología
- **Sismos Híbridos (Global + Local):** Combina el feed global del **USGS** con el feed XML oficial del **INPRES** (Argentina). Un algoritmo propio "deduplica" eventos (si ambas agencias reportan el mismo sismo, lo unifica) y los ordena cronológicamente.
- **Resaltado Inteligente:** Si el sismo ocurrió en las últimas 24 horas, parpadea sutilmente en rojo para llamar la atención.

### Finanzas 24/7
- **Cotizaciones Dólar:** Integrado con DolarAPI para mostrar el Dólar Blue. Además, incluye el **Dólar Cripto**, lo que garantiza que el dashboard muestre movimientos financieros incluso durante fines de semana y feriados.
- **Bitcoin:** Se conecta a APIs cripto para mostrar el precio en tiempo real del BTC y calcula su tendencia (alza o baja en verde/rojo) respecto a las últimas 24 horas.

---

## Configuración Individual (Tuerquita)
El diseño respeta al usuario. Cada dispositivo puede ajustar sus preferencias de forma local:
- **Temas Visuales:** Soporte nativo para DaisyUI. Cambia toda la interfaz a temas como *Dracula, Synthwave, Cyberpunk, Retro*, etc.
- **Visibilidad:** Activa o desactiva de forma modular las tarjetas de Fútbol, F1 o UFC si no te interesan.
- **Filtros Inteligentes:** Escribe el nombre de tu equipo de fútbol favorito (con autocompletado) y el sistema siempre priorizará mostrar ese partido. Lo mismo para el clima: escribe tu ciudad y obtendrás tus datos meteorológicos.

## Despliegue (Docker)
El proyecto está listo para producción mediante `docker-compose`. 
Al iniciar, el sistema realiza una carga síncrona de datos ("Pre-update de caches") asegurándose de que cuando aceptes la primera conexión HTTP, el dashboard ya esté completamente renderizado y poblado de datos, sin tiempos de espera.
