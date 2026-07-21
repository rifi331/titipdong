# syntax=docker/dockerfile:1
# TitipDong — multi-stage build.
# Stage 1: build Tailwind CSS. Stage 2: build Go binary with embedded assets.
# Stage 3: tiny runtime image.

# ---------- Stage 1: CSS ----------
FROM node:20-alpine AS css
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci || npm install
COPY tailwind.config.js ./
COPY web/static/src.css ./web/static/src.css
COPY internal/web/templates ./internal/web/templates
RUN npx tailwindcss -i ./web/static/src.css -o ./web/static/app.css --minify

# ---------- Stage 2: Go build ----------
# Pin Go to match the go.mod toolchain directive (avoids "go mod download"
# failing in CI because the image's Go is older than the module requires).
FROM golang:1.25-alpine AS go
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Overwrite the dev CSS with the production build from stage 1.
COPY --from=css /app/web/static/app.css ./web/static/app.css
# Inject the git ref (tag/branch/sha) as version.Version at build time.
# github.ref is refs/tags/v0.4.0 -> version string "v0.4.0"; for branch pushes
# it's refs/heads/main -> "main-<sha>". Falls back to "dev" for local builds.
ARG APP_VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X github.com/titipdong/titipdong/internal/version.Version=${APP_VERSION}" \
    -o /out/titipdong ./cmd/titipdong

# ---------- Stage 3: runtime ----------
# Use alpine (not distroless) so we can chown the uploads dir to the non-root
# user. Distroless has no shell, which made /app/uploads unwritable for the
# nonroot user — photos silently failed to save.
FROM alpine:3.20
RUN adduser -D -u 65532 nonroot
WORKDIR /app
COPY --from=go /out/titipdong /app/titipdong
# Templates are embedded at build time, but we also ship static assets that
# are served from disk (JS, compiled CSS, manifest, icons, service worker).
COPY web/static /app/web/static
# Create the uploads dir owned by nonroot so photos can be written at runtime.
# The docker volume mounts OVER this path; the ownership here is the fallback
# for when the volume is empty (first boot).
RUN mkdir -p /app/uploads && chown -R nonroot:nonroot /app/uploads
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/titipdong"]
