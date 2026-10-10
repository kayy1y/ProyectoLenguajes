# Contexto de build: raíz del repositorio (necesita contracts/fixtures).
FROM golang:1.26-alpine AS build
WORKDIR /src/simulator
COPY simulator/go.mod ./
RUN go mod download
COPY simulator/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/simulator ./cmd/simulator

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/simulator /app/simulator
COPY contracts/fixtures /app/contracts/fixtures
ENV FIXTURES_DIR=/app/contracts/fixtures
ENTRYPOINT ["/app/simulator"]
