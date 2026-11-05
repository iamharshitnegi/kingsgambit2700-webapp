# syntax=docker/dockerfile:1.7
FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates git tzdata
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/realtime ./cmd/realtime
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/river-migrate ./cmd/river-migrate
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/bootstrap-admin ./cmd/bootstrap-admin

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates tzdata && addgroup -S -g 65532 app && adduser -S -D -H -u 65532 -G app app
WORKDIR /app
USER 65532:65532

FROM runtime AS api
COPY --from=build /out/api /app/api
COPY migrations /app/migrations
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/health/live || exit 1
ENTRYPOINT ["/app/api"]

FROM runtime AS worker
COPY --from=build /out/worker /app/worker
ENTRYPOINT ["/app/worker"]

FROM runtime AS river-migrate
COPY --from=build /out/river-migrate /app/river-migrate
ENTRYPOINT ["/app/river-migrate"]

FROM runtime AS bootstrap-admin
COPY --from=build /out/bootstrap-admin /app/bootstrap-admin
ENTRYPOINT ["/app/bootstrap-admin"]

FROM runtime AS realtime
COPY --from=build /out/realtime /app/realtime
EXPOSE 8081
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8081/health/live || exit 1
ENTRYPOINT ["/app/realtime"]
