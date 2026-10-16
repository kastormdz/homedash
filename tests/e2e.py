#!/usr/bin/env python3
"""E2E del panel homedash. Se corre ANTES de deployar: si esto pasa, se deploya.

Uso:
    python3 tests/e2e.py                        # contra http://127.0.0.1:8060
    python3 tests/e2e.py --url https://home.cronix.com.ar
    python3 tests/e2e.py --lento                # incluye esperar el refresh real (70s)

Requiere playwright (el venv de Hermes lo tiene):
    /home/samba/docker/hermes/.hermes/hermes-agent/venv/bin/python tests/e2e.py

Que cubre, y por que:

  HTTP/estructura  los bloques del panel estan y la pagina responde 200 en las 4 vistas.
  Consola JS       un error de JS rompe htmx o lucide y los modales quedan vacios sin
                   que se vea nada raro en el HTML.
  Imagenes         `naturalWidth == 0` distingue un `src` que da 404 (dato) de un
                   template roto (markup). Es la medicion que sirve para "no sale
                   una imagen".
  Modales          los 3 modales abren y traen contenido. Se abren ANTES del fetch
                   (htmx 1.9.10 falla en silencio con un target de 0x0).
  Refresh          `#live` sigue en el DOM con los triggers puestos y `#tick` es
                   HERMANO (no ancestro): si un `hx-select` cae en un ancestro, los
                   hx-get de los hijos heredan el select y el swap queda vacio.
  Sin oklch        emula un WebView que no parsea oklch (Chrome < 111, el ultimo
                   WebView de Android 6 es el 107). El panel se pinta entero con
                   oklch(var(--b*)): sin fallback queda texto oscuro sobre fondo
                   oscuro. Se mide el CONTRASTE real, no se mira la captura.
"""
import argparse
import os
import re
import sys
import time

try:
    from playwright.sync_api import sync_playwright
except ImportError:
    sys.exit("falta playwright: usar el venv de Hermes "
             "(/home/samba/docker/hermes/.hermes/hermes-agent/venv/bin/python)")

OKLCH = r'oklch\((?:[^()]|\([^()]*\))*\)'
FB = re.compile(r'var\((--fb-[a-z0-9-]+),\s*' + OKLCH + r'\)')
DECL = re.compile(r'[a-zA-Z-]+\s*:\s*[^;{}]*' + OKLCH + r'[^;{}]*;?')


class E2E:
    def __init__(self):
        self.ok = 0
        self.fallos = []

    def check(self, nombre, cond, detalle=""):
        if cond:
            self.ok += 1
            print(f"  \033[32mOK\033[0m   {nombre}" + (f"  · {detalle}" if detalle else ""))
        else:
            self.fallos.append(nombre)
            print(f"  \033[31mFALLA\033[0m {nombre}" + (f"  · {detalle}" if detalle else ""))
        return cond

    def resumen(self):
        print()
        if self.fallos:
            print(f"  RESULTADO: {len(self.fallos)} FALLA(S) de {self.ok + len(self.fallos)}")
            for f in self.fallos:
                print(f"     - {f}")
            return 1
        print(f"  RESULTADO: {self.ok}/{self.ok} OK")
        return 0


def ratio(c1, c2):
    def lum(c):
        def f(v):
            v /= 255.0
            return v / 12.92 if v <= 0.03928 else ((v + 0.055) / 1.055) ** 2.4
        return 0.2126 * f(c[0]) + 0.7152 * f(c[1]) + 0.0722 * f(c[2])
    a, b = lum(c1), lum(c2)
    hi, lo = max(a, b), min(a, b)
    return (hi + 0.05) / (lo + 0.05)


def rgb(s):
    m = re.match(r'rgba?\(([\d.]+),\s*([\d.]+),\s*([\d.]+)', s or '')
    return [float(m.group(i)) for i in (1, 2, 3)] if m else None


def leer_fallbacks(repo):
    """Los --fb-* pueden estar en el CSS y usarse en el <style> del HTML: hay que
    juntarlos de TODOS los recursos, porque el navegador los resuelve en conjunto."""
    fb = {}
    for rel in ("static/test2.css", "templates/test2_b.html"):
        p = os.path.join(repo, rel)
        if os.path.exists(p):
            fb.update(dict(re.findall(r'(--fb-[a-z0-9-]+)\s*:\s*(#[0-9A-Fa-f]{6})',
                                      open(p, encoding="utf-8").read())))
    return fb


def emular_sin_oklch(txt, fallbacks):
    """Lo que hace un navegador sin oklch: usa el fallback si existe, descarta si no."""
    txt = FB.sub(lambda m: fallbacks.get(m.group(1), 'initial'), txt)
    return DECL.sub('', txt)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--url", default="http://127.0.0.1:8060")
    ap.add_argument("--repo", default=os.getcwd())
    ap.add_argument("--lento", action="store_true", help="espera el refresh real (70s)")
    ap.add_argument("--capturas", default="")
    a = ap.parse_args()

    t = E2E()
    base = a.url.rstrip("/")
    fallbacks = leer_fallbacks(a.repo)
    print(f"\n  E2E homedash — {base}")
    print(f"  fallbacks sin oklch disponibles: {len(fallbacks)}\n")

    with sync_playwright() as p:
        b = p.chromium.launch(headless=True)

        # ---------- 1. HTTP y estructura ----------
        print("  --- HTTP y estructura ---")
        for vista, ruta in (("home", "/"), ("weather", "/weather"),
                            ("test2 a", "/test2?v=a"), ("test2 b", "/test2?v=b"),
                            ("test2 c", "/test2?v=c")):
            r = b.new_page().goto(f"{base}{ruta}", wait_until="domcontentloaded", timeout=60000)
            t.check(f"{vista} responde 200", r is not None and r.status == 200,
                    f"HTTP {r.status if r else '?'}")

        pg = b.new_page(viewport={"width": 1500, "height": 950})
        errs = []
        pg.on("pageerror", lambda e: errs.append(str(e)[:120]))
        pg.goto(f"{base}/", wait_until="networkidle", timeout=90000)
        pg.wait_for_timeout(3000)

        d = pg.evaluate("""() => ({
          live: !!document.querySelector('#live'),
          tick: !!document.querySelector('#tick'),
          bar: !!document.querySelector('.bar'),
          toprow: !!document.querySelector('.toprow'),
          wrap: !!document.querySelector('.wrap'),
          kpis: document.querySelectorAll('.kpi').length,
          mkt: document.querySelectorAll('.mkt').length,
          texto: document.body.innerText.length,
        })""")
        t.check("el panel tiene sus bloques (#live, .bar, .toprow, .wrap)",
                d['live'] and d['bar'] and d['toprow'] and d['wrap'])
        t.check("los KPIs y los mercados estan", d['kpis'] >= 3 and d['mkt'] >= 8,
                f"{d['kpis']} KPIs, {d['mkt']} mercados")
        t.check("la pagina tiene contenido", d['texto'] > 800, f"{d['texto']} chars")
        t.check("sin errores de JS", not errs, f"{len(errs)} errores" if errs else "")

        imgs = pg.evaluate("""() => ({tot: document.images.length,
          rotas: [...document.images].filter(i => !i.naturalWidth).map(i => i.src.slice(-40))})""")
        t.check("ninguna imagen rota", not imgs['rotas'],
                f"{imgs['tot']} imagenes" + (f", rotas: {imgs['rotas']}" if imgs['rotas'] else ""))

        # ---------- 2. refresh ----------
        print("\n  --- refresh (htmx) ---")
        r = pg.evaluate("""() => {
          const tick = document.querySelector('#tick');
          const live = document.querySelector('#live');
          return {tickSibling: tick && live ? !live.contains(tick) && !tick.contains(live) : false,
                  trigger: tick ? (tick.getAttribute('hx-trigger')||'') : '',
                  oob: live ? (live.getAttribute('hx-swap-oob')||'') : ''};
        }""")
        t.check("#tick es HERMANO de #live (no ancestro)", r['tickSibling'])
        t.check("#tick tiene el trigger del refresh", 'every' in r['trigger'], r['trigger'])
        t.check("#live se reemplaza con hx-swap-oob", r['oob'] != '', f"hx-swap-oob={r['oob']!r}")

        if a.lento:
            h1 = pg.evaluate("() => document.querySelector('.bar .clock').innerText.split('\\n')[0]")
            pg.wait_for_timeout(70000)
            h2 = pg.evaluate("() => document.querySelector('.bar .clock').innerText.split('\\n')[0]")
            t.check("el reloj se actualiza solo (70s)", h1 != h2, f"{h1} -> {h2}")

        # ---------- 3. modales ----------
        print("\n  --- modales ---")
        # Los modales se disparan con un CLICK real: el boton tiene hx-get con
        # hx-trigger="click delay:10ms", asi que llamar abrirModalPartidos() por JS solo
        # abre el modal y htmx nunca pide los datos (quedaba el placeholder de 202 B).
        for nombre, selector, cont, minimo in (
                ("partidos", "button:has-text('Partidos del día')", "#partidos-content", 800),
                ("F1", "button:has-text('Posiciones')", "#f1-content", 800)):
            pg.locator(selector).first.click()
            pg.wait_for_timeout(5000)
            n = pg.evaluate(f"() => (document.querySelector('{cont}')||{{}}).innerHTML?.length || 0")
            t.check(f"el modal de {nombre} trae contenido", n > minimo, f"{n} B")
            pg.evaluate("() => { document.querySelectorAll('.custom-modal').forEach(m => m.classList.remove('active')); }")
            pg.wait_for_timeout(600)

        # ---------- 4. sin oklch ----------
        print("\n  --- sin oklch (WebView <= 110) ---")
        if not fallbacks:
            t.check("hay fallbacks definidos para oklch", False,
                    "0 fallbacks --fb-*: el panel queda ilegible sin oklch")
        for variante in ("a", "b", "c"):
            pg2 = b.new_page(viewport={"width": 1500, "height": 950})

            # OJO: Playwright llama handler(route, request). Un default en el segundo
            # parametro NO sirve: se pisa con el Request.
            def hacer_handler(fb):
                def handler(route, request):
                    resp = route.fetch()
                    ct = (resp.headers.get("content-type") or "")
                    if "css" in ct or "html" in ct:
                        route.fulfill(body=emular_sin_oklch(resp.text(), fb),
                                      headers={**resp.headers,
                                               "content-type": ("text/css" if "css" in ct
                                                                else "text/html; charset=utf-8")})
                    else:
                        route.fulfill(response=resp)
                return handler

            pg2.route(re.compile(r"^https?://[^/]+/.*"), hacer_handler(fallbacks))
            pg2.goto(f"{base}/test2?v={variante}", wait_until="networkidle", timeout=90000)
            pg2.wait_for_timeout(2500)
            d2 = pg2.evaluate("""() => {
              const out = [];
              const vis = e => { const b = e.getBoundingClientRect(); return b.width > 40 && b.height > 6; };
              const add = (sel, etq) => { for (const e of document.querySelectorAll(sel)) {
                if (!vis(e)) continue;
                let n = e, bg = 'rgba(0, 0, 0, 0)';
                while (n && (bg === 'rgba(0, 0, 0, 0)' || bg === 'transparent')) {
                  bg = getComputedStyle(n).backgroundColor; n = n.parentElement; }
                out.push({color: getComputedStyle(e).color, bg}); } };
              add('body','body'); add('.bar .clock','reloj'); add('.kpi .k','etq');
              add('.kpi .v','val'); add('.stitle .lbl','titulo'); add('.mkt .nom','mercado');
              return out;
            }""")
            peor = 99.0
            for it in d2:
                c, bg = rgb(it['color']), rgb(it['bg'])
                if not c or not bg:
                    continue
                peor = min(peor, ratio(c, bg))
            t.check(f"variante {variante}: contraste >= 3:1 sin oklch", peor >= 3.0,
                    f"peor {peor:.2f}:1")
            pg2.close()

        # ---------- capturas ----------
        if a.capturas:
            os.makedirs(a.capturas, exist_ok=True)
            pg.screenshot(path=os.path.join(a.capturas, "e2e-home.png"),
                          clip={"x": 0, "y": 0, "width": 1500, "height": 640})
            print(f"\n  captura: {a.capturas}/e2e-home.png")

        b.close()

    return t.resumen()


if __name__ == "__main__":
    sys.exit(main())
