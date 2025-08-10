# -- Etapa 1: Compilación --
FROM golang:1.22-alpine3.19 AS builder

WORKDIR /app
ENV GOCACHE=/root/.cache/go-build

# Copiar primero los archivos de definición de módulos.
COPY go.mod go.sum ./

# Descargar las dependencias.
RUN  --mount=type=cache,target=/root/.cache/go-build go mod download

# Copiar el resto del código fuente de la aplicación.
COPY . .

# Compilar el binario.
RUN  --mount=type=cache,target=/root/.cache/go-build go build -o /go/bin/mango .

# -- Etapa 2: Imagen Final --
FROM alpine:3.19

# Instalar procps (para gestión de procesos) y ruby (para el script).
RUN apk add procps ruby bash tini

WORKDIR /app

# Copiar el binario de Mango desde la etapa de compilación.
COPY --from=builder /go/bin/mango /app/mango

# Copiar el directorio de fixtures que contiene el Procfile y el script.
# Se asume que en su contexto de compilación existe la carpeta "fixtures".
COPY fixtures/ ./fixtures/

ENTRYPOINT ["/sbin/tini","--"]


# El comando de inicio ahora ejecuta Mango con el Procfile del directorio fixtures.
CMD ["./mango", "start", "-f", "fixtures/forever/Procfile"]