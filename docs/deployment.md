# Deployment

## Resumen

| Pieza | Cómo |
|---|---|
| Imágenes | `release.yml` publica `ghcr.io/<owner>/gamestats-{ingestion,analytics,simulator}` para amd64 y arm64 en cada push a `main` (etiquetas `latest` y `sha-<commit>`) y en cada tag `v*` |
| Servidor | Una VM Linux pequeña con Docker y `deploy/docker-compose.deploy.yml` |
| Despliegue | **Manual y documentado** (abajo), o con el botón *Run workflow* de `deploy.yml` si se configuran los secrets. No es un despliegue automático en cada push |
| Exposición | Solo el puerto 8080 (ingestion). Scala queda en la red interna de Docker. Las escrituras exigen `X-API-Key` |

> CI = compilar, probar y construir imágenes en cada cambio (automático).
> CD de artefactos = publicar imágenes en GHCR (automático).
> Despliegue en el servidor = manual o con un clic.

## Opciones de servidor

| Opción | Costo | Notas |
|---|---|---|
| **VM con créditos estudiantiles** (p. ej. DigitalOcean o Azure vía GitHub Student Developer Pack) | 0 con créditos | Recomendada. 1 vCPU / 1 GB alcanza con `JAVA_OPTS=-Xmx256m` y 1 GB de swap. Verificar que la oferta siga vigente |
| Oracle Cloud Always Free (ARM) | 0 | Las imágenes ya son arm64; dar de alta la cuenta a veces falla |
| GCP e2-micro / AWS free tier | 0 o bajo | Revisar las condiciones vigentes del free tier |
| Máquina propia o del laboratorio con túnel (`cloudflared tunnel --url http://localhost:8080`) | 0 | Respaldo; **confirmar con el docente** que lo acepta como deployment |

## Pasos (VM Ubuntu)

```bash
# 1) En la VM: Docker Engine + plugin compose
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER && newgrp docker
sudo fallocate -l 1G /swapfile && sudo chmod 600 /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile

# 2) Firewall: SSH (idealmente solo tu IP) y 8080
sudo ufw allow OpenSSH && sudo ufw allow 8080/tcp && sudo ufw enable

# 3) Archivos de despliegue
mkdir -p ~/gamestats && cd ~/gamestats
curl -fsSLO https://raw.githubusercontent.com/<owner>/proyectolenguajes/main/deploy/docker-compose.deploy.yml
curl -fsSL  https://raw.githubusercontent.com/<owner>/proyectolenguajes/main/deploy/.env.deploy.example -o .env
nano .env   # GHCR_OWNER, IMAGE_TAG y una INGEST_API_KEY larga y aleatoria

# 4) Si el repositorio es privado, las imágenes de GHCR también lo son: iniciar sesión con un token (read:packages)
echo "<token>" | docker login ghcr.io -u <usuario> --password-stdin

# 5) Arrancar
docker compose -f docker-compose.deploy.yml pull
docker compose -f docker-compose.deploy.yml up -d --wait
```

`restart: unless-stopped` y Docker habilitado en systemd hacen que los servicios vuelvan solos tras un reinicio de la VM.

## Comprobación desde fuera

```bash
curl http://<ip>:8080/healthz
cd simulator && go run ./cmd/simulator --target http://<ip>:8080 --api-key <clave> \
  --scenario demo --seed 42 --fixtures-dir ../contracts/fixtures
curl -s http://<ip>:8080/v1/matches | jq
```

## Actualizar, reiniciar y volver atrás

```bash
# Actualizar a una versión concreta (o usar deploy.yml con la etiqueta)
sed -i 's/^IMAGE_TAG=.*/IMAGE_TAG=sha-abc1234/' .env
docker compose -f docker-compose.deploy.yml pull && docker compose -f docker-compose.deploy.yml up -d --wait

# Volver atrás: la misma operación con la etiqueta anterior (están todas en GHCR)
# Reiniciar un servicio
docker compose -f docker-compose.deploy.yml restart ingestion
# Logs
docker compose -f docker-compose.deploy.yml logs -f ingestion
```

Los resultados analizados persisten en el volumen `gamestats-data` (ver `docs/architecture.md` §6).

## Deploy con un clic (opcional)

En *Settings → Secrets and variables → Actions* del repositorio:

| Secret | Valor |
|---|---|
| `DEPLOY_HOST` | IP o dominio de la VM |
| `DEPLOY_USER` | usuario SSH |
| `DEPLOY_SSH_KEY` | clave privada SSH, dedicada a esto |

Después, en *Actions → Deploy → Run workflow*, se indica la etiqueta. El workflow actualiza `.env`, descarga, reinicia y comprueba `/healthz` desde fuera.

## Evidencias a guardar en `docs/evidence/`

- Captura de *Actions* con CI en verde (jobs `go`, `scala`, `docker`, `e2e`) y de *Release images*.
- Captura de los paquetes en GHCR.
- En la VM: `docker compose -f docker-compose.deploy.yml ps`.
- Desde la laptop: salida de `curl http://<ip>:8080/healthz` y del simulador contra la IP pública.
- JSON de un resultado remoto (`curl …/results | jq`).
