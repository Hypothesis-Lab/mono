FROM golang:1.26-alpine3.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o /app/bin/app ./cmd/app


FROM alpine:3.24

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/bin/app /app/app

ENTRYPOINT ["/app/app"]