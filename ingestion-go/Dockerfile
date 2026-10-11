# Contexto de build: raíz del repositorio (necesita contracts/).
FROM golang:1.26-alpine AS build
WORKDIR /src/ingestion-go
COPY ingestion-go/go.mod ingestion-go/go.sum ./
RUN go mod download
COPY ingestion-go/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ingestion ./cmd/ingestion

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/ingestion /app/ingestion
COPY contracts/schemas /app/contracts/schemas
ENV HTTP_ADDR=:8080 \
    SCHEMAS_DIR=/app/contracts/schemas
EXPOSE 8080
# Distroless no tiene shell ni wget: el binario se comprueba a sí mismo.
HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=12 CMD ["/app/ingestion", "-healthcheck"]
ENTRYPOINT ["/app/ingestion"]
