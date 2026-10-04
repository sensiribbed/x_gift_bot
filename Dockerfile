# syntax=docker/dockerfile:1
ARG NODE_IMAGE=node:22-bookworm-slim
ARG GO_IMAGE=golang:1.27.1-bookworm
FROM ${NODE_IMAGE} AS frontend
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY tsconfig.json ./
COPY frontend ./frontend
RUN mkdir -p internal/site/assets && npm run build

FROM ${GO_IMAGE} AS backend
WORKDIR /src
ENV CGO_ENABLED=1
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=frontend /src/internal/site/assets/ ./internal/site/assets/
RUN go build -trimpath -o /out/xgift ./cmd/xgift && \
    go build -trimpath -o /out/xgift-web ./cmd/xgift-web

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl tzdata jq && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd --gid 10001 xgift && useradd --uid 10001 --gid xgift --no-create-home xgift && \
    install -d -o xgift -g xgift -m 0700 /data
COPY --from=backend /out/ /usr/local/bin/
COPY --chmod=755 deploy/docker/entrypoint.sh /usr/local/bin/docker-entrypoint
COPY --chmod=755 deploy/docker/bootstrap.sh /usr/local/bin/xgift-bootstrap
USER 10001:10001
WORKDIR /data
ENTRYPOINT ["docker-entrypoint"]
CMD ["xgift-web"]
