# 1단계: 빌드 스테이지
FROM golang:1.24-alpine AS builder

# 필요한 패키지 설치
RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /worker ./cmd/worker

# 2단계: 실행 스테이지 (Go 필요: metaflow_manager 등 go run 명령 실행)
FROM golang:1.24-alpine

RUN apk add --no-cache ca-certificates docker-cli openssh-client git

WORKDIR /
COPY --from=builder /worker /worker

ENV TEMPORAL_ADDRESS=temporal:7233

ENTRYPOINT ["/worker"]