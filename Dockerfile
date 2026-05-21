### STAGE 1: Compilación (Builder)
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache tzdata

WORKDIR /src

# 1. Cache de módulos (No cambia)
COPY go.mod go.sum ./
RUN go mod download

# 2. Copia consolidada del código fuente (Ahorra ~3-5s de I/O)
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# 3. COMPILACIÓN CON CACHE MOUNTS
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /homedash ./cmd/

### STAGE 2: Producción (Imagen final)
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="homedash" \
      org.opencontainers.image.description="Homelab Dashboard"

WORKDIR /app

# Copiamos la base de datos de zonas horarias
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# 4. Copias directas con ownership (Más rápido que el hop del builder)
COPY --chown=nonroot:nonroot --from=builder /homedash .
COPY --chown=nonroot:nonroot static/ ./static/
COPY --chown=nonroot:nonroot templates/ ./templates/

USER nonroot:nonroot

ENV TZ=America/Argentina/Buenos_Aires
EXPOSE 8060

CMD ["./homedash"]
