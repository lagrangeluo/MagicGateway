# ---- Build stage ----
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags="-s -w -X main.Version=$VERSION" -o gateway .

# ---- Runtime stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app
COPY --from=builder /src/gateway .
COPY config.yaml .

RUN mkdir -p /app/data
VOLUME ["/app/data"]

EXPOSE 8080

CMD ["./gateway"]
