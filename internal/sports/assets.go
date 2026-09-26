package sports

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Los trazados de circuitos y las banderas que no vienen en el repo se bajan de la
// fuente externa y se dejan en static/assets/cache/. Ese es el único directorio
// escribible por el contenedor (corre como UID 65532 y el compose le monta un bind rw):
// static/assets/circuits viaja dentro de la imagen y se perdería en cada recreate.
//
// Si el asset no se puede traer, el llamador devuelve "": el template omite el <img>.
// Eso importa porque /static/ se sirve con Cache-Control de 24h: un <img> a una URL que
// devuelve 404 también se cachea en el navegador, así que una URL que sabemos rota es
// peor que no pintar nada.
//
// Orden de búsqueda: primero el repo (viene en la imagen, es el trazado "oficial"), y
// después la caché. Así, si mañana se agrega el SVG al repo, ese gana sin tocar nada.

const (
	timeoutDescarga = 10 * time.Second
	reintentoFallo  = 6 * time.Hour
)

var (
	fallosMu sync.Mutex
	fallos   = map[string]time.Time{}

	descargaMu sync.Mutex
)

// rutaEnDisco busca el asset en el repo y después en la caché, y devuelve la URL
// pública (la que sirve http.FileServer sobre static/) junto con la ruta en disco.
func rutaEnDisco(sub, nombre string) (url, ruta string) {
	repo := filepath.Join("static", "assets", sub, nombre)
	if _, err := os.Stat(repo); err == nil {
		return "/static/assets/" + sub + "/" + nombre, repo
	}
	cache := filepath.Join("static", "assets", "cache", sub, nombre)
	if _, err := os.Stat(cache); err == nil {
		return "/static/assets/cache/" + sub + "/" + nombre, cache
	}
	return "", ""
}

// enEsperaDeReintento dice si un asset que falló todavía está en su ventana de castigo.
// Sin esto, un circuito que la fuente no tiene se reintentaría (y pagaría el timeout)
// en cada actualización del feed.
func enEsperaDeReintento(clave string) bool {
	fallosMu.Lock()
	defer fallosMu.Unlock()
	t, ok := fallos[clave]
	return ok && time.Since(t) < reintentoFallo
}

func marcarFallo(clave string) {
	fallosMu.Lock()
	fallos[clave] = time.Now()
	fallosMu.Unlock()
}

func limpiarFallo(clave string) {
	fallosMu.Lock()
	delete(fallos, clave)
	fallosMu.Unlock()
}

// asegurar devuelve la URL del asset, trayéndolo si no está en disco. traer() se encarga
// de bajarlo y dejarlo en la caché.
//
// Serializado a propósito: dos goroutines que piden el mismo asset no lo bajan dos veces.
// El chequeo se repite después de tomar el lock porque otra goroutine pudo haberlo
// resuelto mientras esperábamos.
func asegurar(sub, nombre string, traer func() bool) string {
	if url, _ := rutaEnDisco(sub, nombre); url != "" {
		return url
	}

	clave := sub + "/" + nombre
	if enEsperaDeReintento(clave) {
		return ""
	}

	descargaMu.Lock()
	defer descargaMu.Unlock()

	if url, _ := rutaEnDisco(sub, nombre); url != "" {
		return url
	}
	if !traer() {
		marcarFallo(clave)
		log.Printf("[ASSETS] no pude traer %s; se omite y reintento en %s", clave, reintentoFallo)
		return ""
	}
	url, _ := rutaEnDisco(sub, nombre)
	log.Printf("[ASSETS] %s traído y cacheado", clave)
	return url
}

func descargar(url string) ([]byte, error) {
	cliente := &http.Client{Timeout: timeoutDescarga}
	resp, err := cliente.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	// 2 MB es de sobra para un SVG de trazado o una bandera.
	return io.ReadAll(io.LimitReader(resp.Body, 2<<20))
}

func guardarCache(sub, nombre string, cuerpo []byte) bool {
	dir := filepath.Join("static", "assets", "cache", sub)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("[ASSETS] no pude crear %s: %v", dir, err)
		return false
	}
	if err := os.WriteFile(filepath.Join(dir, nombre), cuerpo, 0o644); err != nil {
		log.Printf("[ASSETS] no pude escribir %s: %v", nombre, err)
		return false
	}
	return true
}

// traerTrazado baja el trazado de julesr0y/f1-circuits-svg: es la misma fuente de la que
// salieron los que ya están en el repo, así que el estilo coincide (500x500, sin relleno
// y stroke blanco de 20).
func traerTrazado(id string) bool {
	// OJO: `details` es obligatorio. Sin ese parametro la fuente responde
	// 400 "Missing parameters" (con curl y la URL completa daba 200, asi que el
	// fallo solo se veia desde el binario).
	url := fmt.Sprintf("https://f1-circuits-svg.alwaysdata.net/download?layoutId=%s-1&details=minimal&style=white-outline", id)
	cuerpo, err := descargar(url)
	if err != nil {
		log.Printf("[ASSETS] trazado %q: %v", id, err)
		return false
	}
	if !strings.Contains(string(cuerpo), "<svg") {
		log.Printf("[ASSETS] trazado %q: la respuesta no es un svg (%d B)", id, len(cuerpo))
		return false
	}
	return guardarCache("circuits", id+".svg", quitarTrazoNegro(cuerpo))
}

// El render de esa fuente trae un segundo path (stroke #000 de 5px) que a 500px se lee
// como doble contorno; los trazados del repo tienen uno solo.
var reTrazoNegro = regexp.MustCompile(`(?s)\s*<path[^>]*stroke:#000[^>]*/>`)

func quitarTrazoNegro(svg []byte) []byte {
	return reTrazoNegro.ReplaceAll(svg, nil)
}

// traerBandera baja la bandera de flagcdn: SVG estándar, el mismo tipo de archivo que
// las que ya están en static/assets/flags.
func traerBandera(iso string) bool {
	cuerpo, err := descargar("https://flagcdn.com/" + iso + ".svg")
	if err != nil {
		log.Printf("[ASSETS] bandera %q: %v", iso, err)
		return false
	}
	if !strings.Contains(string(cuerpo), "<svg") {
		log.Printf("[ASSETS] bandera %q: la respuesta no es un svg (%d B)", iso, len(cuerpo))
		return false
	}
	return guardarCache("flags", iso+".svg", cuerpo)
}
