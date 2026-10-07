FROM docker.io/library/node:26-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM docker.io/library/caddy:alpine

COPY Caddyfile /etc/caddy/Caddyfile
COPY --from=builder /app/dist /srv
