FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# 의존성 파일 복사 및 다운로드
COPY go.mod go.sum* ./
RUN go mod download

# 소스 코드 복사
COPY . .

# 바이너리 빌드
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o collector \
    ./cmd/collector/main.go

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /build/collector /app/collector

RUN chown -R appuser:appuser /app

USER appuser

ENTRYPOINT ["/app/collector"]

