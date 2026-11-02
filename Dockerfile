# One image, one URL: Go serves the API and the built React app from the same
# origin, so session cookies stay first-party and no CORS is needed.

# ---- 1. build the React frontend ----
FROM node:22-alpine AS web
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ---- 2. build the Go backend ----
FROM golang:1.26-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- 3. small runtime image ----
FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -H app
WORKDIR /app
COPY --from=api /out/server ./server
COPY --from=web /web/dist ./public
ENV APP_ENV=production \
    STATIC_DIR=/app/public \
    PORT=8080
USER app
EXPOSE 8080
CMD ["./server"]
