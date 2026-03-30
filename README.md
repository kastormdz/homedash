# Homedash

Homedash es un dashboard minimalista de información útil (y alguna inútil), optimizado para carga instantánea, personalización individual y visualización de datos en tiempo real. Está diseñado con la filosofía de **"Cero Hardcoding"**, alimentándose completamente de múltiples APIs para mantenerse siempre actualizado sin que tengas que tocar nada.

---

## 🏗️ Arquitectura y Stack Tecnológico (V2.0)

- **Backend:** Go (Golang) con caché atómica en memoria. Implementa propagación de `context` para evitar fugas de memoria y protección mediante `sync.Mutex` para asegurar la integridad de los datos bajo alta concurrencia.
- **Frontend:** **HTMX** para recargas parciales cada 60 segundos. Logramos una experiencia de SPA (Single Page Application) sin la complejidad de los frameworks de JS pesados.
- **Seguridad (Hardening):** 
    - Imagen base **Google Distroless Static** (sin shell, sin gestores de paquetes, mínima superficie de ataque).
    - Ejecución como usuario **nonroot** (UID 65532).
    - Protección **SSRF** con lista blanca de dominios y bloqueo de IPs internas.
    - Headers de seguridad estrictos (**CSP**, **HSTS**, **MIME sniffing prevention**).
    - Integridad de Subrecursos (**SRI**) para todos los scripts y estilos de CDNs externos.
- **UI/UX:** Tailwind CSS + **DaisyUI** para temas dinámicos. Diseño "Zero Scroll" optimizado para que todo entre en una sola pantalla.
- **Persistencia:** Configuración guardada en Cookies codificadas en JSON. Cada dispositivo de tu casa puede tener su propio tema y sus propios equipos sin pisarse con los demás.

---

## 🚀 Funcionalidades Principales

### ⚽ Deportes al Palo
- **Fútbol Argentino e Internacional:** Datos de **Promiedos** y **ESPN**. Muestra resultados en vivo, minutero y detecta el canal de transmisión (TNT Sports, ESPN Premium, etc.). Prioriza siempre el partido de "Tu Equipo".
- **Fórmula 1:** Soporte completo para fines de semana convencionales y de **Sprint** (SQualy). Trae horarios locales de todas las sesiones y el trazado del circuito. **Exclusivo:** Inyecta el pronóstico del clima exacto para las coordenadas del circuito en cada día de actividad.
- **UFC:** Cartelera actualizada vía ESPN. Muestra fotos (headshots) de los peleadores del evento principal y salta automáticamente al próximo evento apenas termina el del sábado.

### 🌦️ Clima y Alertas
- **Pronóstico de 7 Días:** Datos de **Open-Meteo**. Incluye sensación térmica, humedad, índice UV, visibilidad y probabilidad de lluvia.
- **Alertas Inteligentes (SMN + Mendoza):** Conexión al RSS del Servicio Meteorológico Nacional para alertas en toda la Argentina. Si estás en Mendoza, se suma el feed de Contingencias Climáticas (DACC) para alertas de granizo o Zonda.
- **Calidad del Aire (AQI):** Monitoreo en tiempo real con advertencias visuales si el aire no está para salir a correr.
- **Fase Lunar:** Algoritmo astronómico interno (escrito en Go) que calcula la fase exacta y la muestra con su icono sin consultar ninguna API externa.

### 💸 Finanzas 24/7
- **Dólar:** Cotizaciones del Blue y el **Dólar Cripto (USDT)** para que veas qué pasa con el mercado incluso en feriados o fines de semana.
- **Riesgo País:** Seguimiento dinámico de los puntos básicos con indicador de tendencia y porcentaje de cambio diario (vía ArgentinaDatos).
- **Índices y Cripto:** Seguimiento del **S&P 500**, **NASDAQ**, **Bitcoin** y **Ethereum** con sus respectivas tendencias del día.

### 🌍 Sismología e Historia
- **Sismos Híbridos:** Combina el feed global del **USGS** con el XML oficial del **INPRES** (Argentina). Un algoritmo de deduplicación unifica los reportes si ambas agencias detectan el mismo evento.
- **Feriados:** Calendario de feriados nacionales de Argentina, incluyendo los inamovibles, trasladables y turísticos.

---

## 🛠️ Configuración (La Tuerquita)
Desde el panel de ajustes (que persiste entre recargas gracias a estar integrado en el layout base) podés:
- **Cambiar el Tema:** Elegí entre *Dark, Dracula, Synthwave, Cyberpunk, Retro, Sunset, Night*, y más.
- **Modularidad:** Activá o desactivá las tarjetas de Fútbol, F1, UFC o Finanzas a tu gusto.
- **Localización:** Escribí tu ciudad para el clima y tu equipo de fútbol para que el dashboard sepa qué mostrarte primero.

---

## 🐳 Despliegue con Docker
Homedash está pensado para correr en un Homelab detrás de un proxy inverso (como Nginx Proxy Manager).

```bash
docker-compose up -d --build
```

El build está optimizado mediante **Docker BuildKit** y mounts de caché, lo que permite recompilar en segundos si solo cambiaste el código Go, o instantáneamente si solo tocaste un HTML.

---
*Homedash por **kastor** - Hecho con mate y ganas de ver todo de un vistazo.*
