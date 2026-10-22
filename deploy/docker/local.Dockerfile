# syntax=docker/dockerfile:1.7
FROM golang:1.26-alpine AS build
RUN apk add --no-cache git ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,id=gomod,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,id=gomod,target=/go/pkg/mod --mount=type=cache,id=gobuild,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -p 4 -o /out/ ./cmd/backend ./cmd/consumer ./cmd/worker ./cmd/coldemailctl ./cmd/schema-check ./cmd/instance-setup
FROM python:3.12-alpine
RUN apk add --no-cache ca-certificates tzdata postgresql-client && adduser -D -u 1000 coldemail && \
    mkdir -p /data/blobs /data/state /app/data && chown -R coldemail:coldemail /data /app
COPY --from=build /out/ /usr/local/bin/
COPY scripts/fake-pramaan.py scripts/provision-local.sh /app/scripts/
USER coldemail
WORKDIR /app
CMD ["backend"]
