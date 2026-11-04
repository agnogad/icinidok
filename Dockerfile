# syntax=docker/dockerfile:1

FROM node:22-alpine AS web
WORKDIR /src/apps/web
COPY apps/web/package.json apps/web/package-lock.json ./
RUN npm ci
COPY apps/web/ ./
RUN npm run build


FROM golang:1.26-alpine AS api
WORKDIR /src/services/api
COPY services/api/go.mod services/api/go.sum ./
RUN go mod download
COPY services/api/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server


FROM alpine:3.21 AS runtime
RUN apk add --no-cache ca-certificates wget && adduser -D -u 10001 icini
WORKDIR /app
COPY --from=api /out/server /app/server
COPY --from=web /src/apps/web/out /app/web
RUN mkdir -p /data && chown -R icini /data
USER icini
ENV ADDR=:8080 \
    DB_PATH=/data/icinidok.db \
    WEB_DIR=/app/web \
    COOKIE_SECURE=false \
    TRUST_PROXY=true
EXPOSE 8080
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s \
  CMD wget -qO- http://127.0.0.1:8080/api/health || exit 1
CMD ["/app/server"]
