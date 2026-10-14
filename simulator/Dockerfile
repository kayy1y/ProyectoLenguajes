# Contexto de build: raíz del repositorio (necesita contracts/fixtures).
# Compila en la plataforma del runner y cruza a la de destino (multi-arch sin emulación).
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src/simulator
COPY simulator/go.mod simulator/go.sum ./
RUN go mod download
COPY simulator/ ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/simulator ./cmd/simulator

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/simulator /app/simulator
COPY contracts/fixtures /app/contracts/fixtures
COPY contracts/expected /app/contracts/expected
ENV FIXTURES_DIR=/app/contracts/fixtures
ENTRYPOINT ["/app/simulator"]
