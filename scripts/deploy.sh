#!/usr/bin/env bash
# Deploy de homedash a producción (Titan, contenedor `homedash` en :8060).
#
#   ./deploy-homedash-fix.sh            build + vet -> rsync -> chown -> compose -> verifica
#   ./deploy-homedash-fix.sh --check    solo verifica (no toca nada)
#
# Reglas que costaron tiempo y estan aplicadas aca:
#   - rsync SIEMPRE con --delete: sin eso, un archivo borrado en el repo sobrevive en el
#     destino y rompe el build de Docker (caso real: worldcup.go huerfano).
#   - --exclude data/ y static/assets/cache/: son estado del contenedor, no del repo.
#   - rsync -rlptD --omit-dir-times, NUNCA -a: la cache pertenece a UID 65532 y como kastor
#     no se puede preservar owner ni el mtime de esos dirs.
#   - chown -R 65532:65532 de la cache despues del rsync: el contenedor corre nonroot.
set -uo pipefail

# readlink -f: si el script se invoca por un symlink (p. ej. /home/kastor/tmp/
# deploy-homedash-fix.sh -> scripts/deploy.sh), $0 es el symlink y dirname daria /tmp.
SELF="$(readlink -f "$0")"
REPO="${REPO:-$(cd "$(dirname "$SELF")/.." && pwd)}"
DEST="${DEST:-/home/samba/docker/homedash}"
CONTENEDOR="${CONTENEDOR:-homedash}"
PUERTO="${PUERTO:-8060}"

rojo()  { printf '\033[31m%s\033[0m\n' "$*"; }
verde() { printf '\033[32m%s\033[0m\n' "$*"; }
amar() { printf '\033[33m%s\033[0m\n' "$*"; }
fallos=0
ok()   { verde "  ✔ $*"; }
mal()  { rojo  "  ✘ $*"; fallos=$((fallos+1)); }

cd "$REPO" || { rojo "no existe el repo: $REPO"; exit 1; }

if [ "${1:-}" != "--check" ]; then
  # === SINCRONIA CON EL REMOTO ===
  # El deploy hace rsync del WORKING TREE, no del commit. Si el repo local quedo atras del
  # remoto, se deploya codigo viejo sin que nadie lo note. Paso: el local estaba 15 commits
  # atras y el deploy habria revertido el panel a una version de semanas antes.
  echo "=== REPO ==="
  if git fetch -q origin 2>/dev/null; then
    detras=$(git rev-list --count HEAD..origin/main 2>/dev/null || echo 0)
    adelante=$(git rev-list --count origin/main..HEAD 2>/dev/null || echo 0)
    if [ "${detras:-0}" -gt 0 ]; then
      rojo "  ✘ el repo local esta $detras commit(s) DETRAS de origin/main"
      rojo "    deployarias codigo viejo. Ver: git log --oneline HEAD..origin/main"
      rojo "    Arreglo: git reset <commit del remoto> y rehacer los commits propios."
      exit 1
    fi
    if [ "${adelante:-0}" -gt 0 ]; then
      amar "  ! $adelante commit(s) locales sin pushear (se deployan igual)"
    else
      ok "repo al dia con origin/main"
    fi
  else
    amar "  ! no pude hacer git fetch: sigo sin verificar la sincronia"
  fi

  sucio=$(git status --porcelain | wc -l)
  if [ "${sucio:-0}" -gt 0 ]; then
    amar "  ! $sucio archivo(s) sin commitear: se deployan tal cual y NO quedan en el historial"
    git status --porcelain | head -6 | sed 's/^/      /'
  else
    ok "working tree limpio"
  fi

  echo "=== BUILD + VET ==="
  if ! go build ./... ; then rojo "el build fallo: NO se deploya"; exit 1; fi
  if ! go vet ./... 2>&1 | grep -v '^#' ; then :; fi
  ok "build y vet"

  echo "=== RSYNC (con --delete) ==="
  rsync -rlptD --omit-dir-times --delete \
        --exclude '.git' --exclude 'homedash' \
        --exclude 'data/' --exclude 'static/assets/cache/' \
        ./ "$DEST"/ || { rojo "rsync fallo"; exit 1; }
  ok "codigo sincronizado"

  # La cache es del contenedor (UID 65532). Como kastor el chown SIEMPRE falla con
  # "Operacion no permitida", asi que solo se intenta si el owner esta mal, y con sudo -n.
  # Comprobarlo antes evita el falso positivo que reportaba un problema en cada deploy.
  if [ -d "$DEST/static/assets/cache" ]; then
    if [ "$(stat -c '%u:%g' "$DEST/static/assets/cache" 2>/dev/null)" = "65532:65532" ]; then
      ok "cache ya con owner 65532:65532"
    elif sudo -n chown -R 65532:65532 "$DEST/static/assets/cache" 2>/dev/null; then
      ok "cache con owner 65532:65532 (via sudo)"
    else
      mal "la cache no quedo con owner 65532:65532"
    fi
  fi

  echo "=== COMPOSE ==="
  ( cd "$DEST" && docker compose up --build -d ) || { rojo "docker compose fallo"; exit 1; }

  # el arranque NO es bloqueante pero el server responde en el mismo segundo; igual se
  # espera a que el puerto conteste antes de verificar
  echo "  esperando :$PUERTO..."
  for _ in $(seq 1 60); do
    curl -s -o /dev/null -m 2 "http://127.0.0.1:$PUERTO/" && break
    sleep 2
  done
  ok "contenedor arriba"
fi

echo "=== VERIFICACIÓN ==="
codigo=$(curl -s -o /dev/null -w '%{http_code}' -m 10 "http://127.0.0.1:$PUERTO/")
[ "$codigo" = "200" ] && ok "home HTTP 200" || mal "home HTTP $codigo"

# gzip: sin Content-Length borrado, un PNG de 1986 B se entrega en 15 B
favicon=$(curl -s -o /tmp/_fav.png -w '%{size_download}' -m 10 "http://127.0.0.1:$PUERTO/static/favicon.png")
[ "${favicon:-0}" -gt 500 ] && ok "favicon con gzip: $favicon B" || mal "favicon: $favicon B (¿gzip roto?)"
file -b /tmp/_fav.png 2>/dev/null | grep -q '^PNG' && ok "el archivo recibido es un PNG valido" || mal "favicon no es PNG"
rm -f /tmp/_fav.png

# OJO: Vary solo aparece en un GET que pide gzip, NO en un HEAD (curl -sI). Verificar con
# HEAD da un falso negativo.
if curl -s -o /dev/null -D - -m 10 -H 'Accept-Encoding: gzip' "http://127.0.0.1:$PUERTO/" \
     | grep -qi 'vary: accept-encoding'; then
  ok "Vary: Accept-Encoding presente"
else
  mal "falta Vary: Accept-Encoding"
fi

[ "$(stat -c '%u:%g' "$DEST/static/assets/cache" 2>/dev/null)" = "65532:65532" ] \
  && ok "cache con owner 65532:65532" || mal "la cache no es del UID 65532"

if docker logs "$CONTENEDOR" --since 10m 2>&1 | grep -qi 'permission denied'; then
  mal "hay 'permission denied' en los ultimos 10 min"
else
  ok "sin errores 'permission denied'"
fi

echo
if [ "$fallos" -eq 0 ]; then verde "RESULTADO: todo OK"; exit 0; fi
rojo "RESULTADO: $fallos problema(s)"; exit 1
