# Stage 1: Build
FROM golang:1.26-alpine AS builder

ARG VERSION=dev
ARG GITHUB_TOKEN

WORKDIR /app

RUN apk add --no-cache git ca-certificates

RUN git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"

COPY go.mod go.sum ./
RUN GONOSUMDB="github.com/ihsansolusi" GOPRIVATE="github.com/ihsansolusi/*" go mod download

RUN git config --global --unset url."https://${GITHUB_TOKEN}@github.com/".insteadOf 2>/dev/null || true

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GONOSUMDB="github.com/ihsansolusi" GOPRIVATE="github.com/ihsansolusi/*" \
    go build -mod=mod -ldflags="-w -s -X main.Version=${VERSION}" \
    -o bin/policy7 \
    ./cmd/server/ \
    || (go build ./... 2>&1 | head -40 && exit 1)

# Stage 2: Runtime
FROM alpine:3.19

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /app/bin/policy7 .
COPY migrations/ migrations/
COPY scripts/ scripts/

RUN apk --no-cache add ca-certificates postgresql-client curl && \
    chmod +x scripts/docker-entrypoint.sh && \
    curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz -C /usr/local/bin

USER appuser

EXPOSE 8085

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
    CMD wget -qO- http://localhost:8085/health || exit 1

ENTRYPOINT ["./scripts/docker-entrypoint.sh"]
