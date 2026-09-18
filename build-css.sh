#!/usr/bin/env bash
# Compila Tailwind 3.4.17 + DaisyUI 4.10.1 a static/app.css y sincroniza los
# ?v= de layout.html con el hash de cada archivo.
#
# Por qué existe: el dashboard usaba los CDN (cdn.tailwindcss.com compila el CSS
# en el NAVEGADOR + daisyui full.min.css = 2,15 MB descomprimidos para ~330
# reglas usadas). Ahora el CSS sale compilado: ~56 KB y sin JIT en runtime.
#
# Los temas (incluidos los custom "nothing" y "terminal") viven en
# tailwind.config.js. Si agregás un tema, tocalo ahí — y sólo ahí.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK="${HD_CSS_WORK:-/home/kastor/tmp/hd-css-build}"
TAILWIND=3.4.17
DAISYUI=4.10.1

echo "==> 1/4 dependencias en $WORK (fuera del repo, no ensucia node_modules)"
mkdir -p "$WORK"
cd "$WORK"
[ -f package.json ] || printf '{"name":"hd-css","private":true}\n' > package.json
if [ ! -d node_modules/tailwindcss ]; then
  npm i --no-audit --no-fund --silent "tailwindcss@$TAILWIND" "daisyui@$DAISYUI"
fi

echo "==> 2/4 copiando insumos (templates + js + go para el scanner de clases)"
rm -rf src && mkdir -p src/templates src/static src/cmd
cp "$REPO"/templates/*.html src/templates/
cp "$REPO"/static/*.js       src/static/
cp "$REPO"/cmd/*.go          src/cmd/
cp "$REPO"/tailwind.config.js "$REPO"/assets/input.css src/

echo "==> 3/4 compilando"
cd src
npx tailwindcss -c tailwind.config.js -i input.css -o app.css --minify
cp app.css "$REPO/static/app.css"
printf '    %s bytes (antes en CDN: ~2.100.000 descomprimidos)\n' "$(stat -c%s "$REPO/static/app.css")"

echo "==> 4/4 cache-busting por hash (nunca más a mano)"
python3 - "$REPO" <<'PY'
import hashlib, re, sys, os
repo = sys.argv[1]
def h(p):
    with open(os.path.join(repo, p), "rb") as f:
        return hashlib.sha256(f.read()).hexdigest()[:8]
tpl = os.path.join(repo, "templates/layout.html")
s = open(tpl, encoding="utf-8").read()
for path, pat in (("static/app.css", r'(/static/app\.css\?v=)[^"]+'),
                  ("static/app.js",  r'(/static/app\.js\?v=)[^"]+')):
    v = h(path)
    s, n = re.subn(pat, lambda m: m.group(1) + v, s)
    if n:
        print(f"    {path} -> ?v={v}")
open(tpl, "w", encoding="utf-8").write(s)
PY

echo
echo "Listo. Si tocaste clases en templates/, volvé a correr esto y hacé commit de:"
echo "  static/app.css  +  templates/layout.html"
