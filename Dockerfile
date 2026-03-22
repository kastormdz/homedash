# STAGE 1: Compilación (Builder)
FROM golang:1.22-alpine AS builder

# Instalamos certificados y git (necesario para módulos de Go)
RUN apk add --no-cache ca-certificates git

WORKDIR /app

# 1. Cache de módulos
COPY go.mod ./
RUN go mod download

# 2. Copia quirúrgica del código fuente
# Al no copiar 'static' ni 'templates' aquí, el build no se invalida si cambias un HTML o una imagen.
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# 3. COMPILACIÓN CON CACHE MOUNTS (BuildKit)
# Esto guarda el cache de compilación de Go entre ejecuciones de Docker.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /homedash ./cmd/main.go

# STAGE 2: Producción (Imagen final)
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copiamos el binario desde el builder
COPY --from=builder /homedash .

# Copiamos los estáticos directamente del contexto de build 
# (Es más rápido que copiarlos desde el stage anterior)
COPY static ./static
COPY templates ./templates

# Ajustes de seguridad y entorno
EXPOSE 8060
ENV TZ=America/Argentina/Buenos_Aires

ENTRYPOINT ["./homedash"]
