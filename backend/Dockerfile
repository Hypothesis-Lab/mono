FROM golang:1.26.9-alpine3.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod \
    go mod download

COPY cmd/ cmd/
COPY datasets/ datasets/
COPY internal/ internal/

RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=go-build,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o /app/bin/app ./cmd/app

FROM alpine:3.24

RUN apk add --no-cache ca-certificates poppler-utils antiword

WORKDIR /app

COPY --from=builder /app/bin/app /app/app
COPY --from=builder /app/datasets /app/datasets

ENTRYPOINT ["/app/app"]
