### STAGE 1: Compilación (Builder)
FROM golang:1.25-alpine AS builder

# Instalamos certificados y git (necesario para módulos de Go)
RUN apk add --no-cache ca-certificates git

WORKDIR /app

# 1. Cache de módulos
COPY go.mod go.sum ./
RUN go mod download

# 2. Copia quirúrgica del código fuente
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# 3. COMPILACIÓN CON CACHE MOUNTS (BuildKit)
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /homedash ./cmd/main.go


### STAGE 2: Producción (Imagen final)
FROM alpine:3.20

LABEL org.opencontainers.image.title="homedash" \
      org.opencontainers.image.version="1.3" \
      org.opencontainers.image.description="Homelab Dashboard"

# Ajustes de seguridad y entorno
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copiamos el binario y assets
COPY --from=builder /homedash .
COPY static/ ./static/
COPY templates/ ./templates/

# Permisos para el usuario no-root
RUN chown -R appuser:appgroup /app
USER appuser

ENV TZ=America/Argentina/Buenos_Aires
EXPOSE 8060

# D3. Healthcheck para monitorear el estado del contenedor
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8060/health || exit 1

CMD ["./homedash"]
