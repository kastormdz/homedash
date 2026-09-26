package sports

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// El binario resuelve static/ relativo a su cwd (en produccion /app). El test corre con el
// cwd en internal/sports, asi que subo a la raiz del repo; si no, todos los paths fallan.
func TestMain(m *testing.M) {
	if _, err := os.Stat("static/assets/circuits"); err != nil {
		if err := os.Chdir("../.."); err != nil {
			fmt.Fprintln(os.Stderr, "no pude subir a la raiz del repo:", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

func TestRutaEnDiscoEncuentraLosDelRepo(t *testing.T) {
	url, ruta := rutaEnDisco("circuits", "monza.svg")
	if url != "/static/assets/circuits/monza.svg" {
		t.Fatalf("url = %q, queria la del repo", url)
	}
	if _, err := os.Stat(ruta); err != nil {
		t.Fatalf("la ruta devuelta no existe: %v", err)
	}
}

// Nunca inventar una URL: si el asset no esta, el llamador tiene que devolver "" para que
// el template omita el <img> (el Cache-Control de /static/ es de 24h y cachearia el 404).
func TestRutaEnDiscoNoInventaURLs(t *testing.T) {
	url, ruta := rutaEnDisco("circuits", "circuito-que-no-existe.svg")
	if url != "" || ruta != "" {
		t.Fatalf("devolvio %q / %q, queria vacio", url, ruta)
	}
}

func TestAsegurarTraeYDespuesUsaLaCache(t *testing.T) {
	const nombre = "_test_asegurar.svg"
	defer os.Remove(filepath.Join("static", "assets", "cache", "circuits", nombre))
	limpiarFallo("circuits/" + nombre)

	llamadas := 0
	traer := func() bool {
		llamadas++
		return guardarCache("circuits", nombre, []byte("<svg></svg>"))
	}

	url := asegurar("circuits", nombre, traer)
	if url != "/static/assets/cache/circuits/"+nombre {
		t.Fatalf("url = %q, queria la de la cache", url)
	}
	if llamadas != 1 {
		t.Fatalf("traer se llamo %d veces, queria 1", llamadas)
	}

	// la segunda vez sale del disco: no vuelve a pedirlo
	if url2 := asegurar("circuits", nombre, traer); url2 != url {
		t.Fatalf("la segunda llamada devolvio %q", url2)
	}
	if llamadas != 1 {
		t.Fatalf("volvio a traer estando en cache (%d llamadas)", llamadas)
	}
}

// Un circuito que la fuente no tiene no puede reintentarse (y pagar el timeout) en cada
// actualizacion del feed.
func TestAsegurarNoReintentaEnBucleCuandoFalla(t *testing.T) {
	const nombre = "_test_falla.svg"
	limpiarFallo("circuits/" + nombre)
	defer limpiarFallo("circuits/" + nombre)

	llamadas := 0
	traer := func() bool { llamadas++; return false }

	for i := 0; i < 3; i++ {
		if url := asegurar("circuits", nombre, traer); url != "" {
			t.Fatalf("intento %d: devolvio %q, queria vacio", i, url)
		}
	}
	if llamadas != 1 {
		t.Fatalf("traer se llamo %d veces: el guard de fallos no esta funcionando", llamadas)
	}
}

// El segundo path (stroke #000 de 5px) del render de la fuente se lee como doble contorno
// a 500px; los trazados del repo tienen uno solo.
func TestQuitarTrazoNegro(t *testing.T) {
	svg := []byte(`<svg><path style="fill:none;stroke:#fff;stroke-width:20" d="M1 1"/>` +
		`<path style="fill:none;stroke:#000;stroke-width:5" d="M2 2"/></svg>`)
	out := string(quitarTrazoNegro(svg))
	if strings.Contains(out, "#000") {
		t.Fatalf("quedo el trazo negro: %s", out)
	}
	if !strings.Contains(out, "#fff") || strings.Count(out, "<path") != 1 {
		t.Fatalf("se llevo el trazo blanco: %s", out)
	}
}

// Camino real: el feed pide un circuito que no tenemos y hay que traerlo de la fuente.
func TestTraerTrazadoReal(t *testing.T) {
	const id = "istanbul" // no esta en static/assets/circuits, si en la fuente
	destino := filepath.Join("static", "assets", "cache", "circuits", id+".svg")
	os.Remove(destino)
	defer os.Remove(destino)
	limpiarFallo("circuits/" + id + ".svg")

	url := GetCircuitData(id)
	if url == "" {
		t.Skip("sin red o la fuente no tiene ese trazado")
	}
	if url != "/static/assets/cache/circuits/"+id+".svg" {
		t.Fatalf("url = %q", url)
	}
	cuerpo, err := os.ReadFile(destino)
	if err != nil {
		t.Fatalf("no quedo en la cache: %v", err)
	}
	if !strings.Contains(string(cuerpo), "<svg") {
		t.Fatalf("lo bajado no es un svg")
	}
	if strings.Contains(string(cuerpo), "stroke:#000") {
		t.Fatalf("quedo el trazo negro")
	}
}

// La bandera del pais del GP (Malasia) no esta en el repo: tiene que bajarla.
func TestTraerBanderaReal(t *testing.T) {
	destino := filepath.Join("static", "assets", "cache", "flags", "my.svg")
	if _, err := os.Stat(destino); err != nil {
		defer os.Remove(destino)
		limpiarFallo("flags/my.svg")
	}

	url := GetFlagURL("Malaysia")
	if url == "" {
		t.Skip("sin red")
	}
	cuerpo, err := os.ReadFile(destino)
	if err != nil {
		t.Fatalf("no quedo en la cache: %v", err)
	}
	if !strings.Contains(string(cuerpo), "<svg") {
		t.Fatalf("lo bajado no es un svg")
	}
}

// Un pais que no conocemos no tiene que caer en una bandera generica.
func TestPaisSinMapearNoDevuelveBandera(t *testing.T) {
	if url := GetFlagURL("Narnia"); url != "" {
		t.Fatalf("devolvio %q, queria vacio", url)
	}
}
