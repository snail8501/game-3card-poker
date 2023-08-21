
# ========================================
# ===== Build image for the golang =====
# ========================================
FROM registry.cn-hongkong.aliyuncs.com/drpool/golang:1.19.0-amd64 AS builder

ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64
ENV CGO_CFLAGS="-g -O2 -Wno-return-local-addr"
ENV GOPROXY https://goproxy.cn
WORKDIR /app

COPY / ./

RUN --mount=type=cache,mode=0777,id=gomod,target=/go/pkg/mod \
    go mod download

RUN --mount=type=cache,mode=0777,target=/root/.cache/go-build \
    --mount=type=cache,mode=0777,id=gomod,target=/go/pkg/mod \
    go mod tidy && go build -tags=embed -o game-3card-poker .

# =======================================
# ===== Build image for the game =====
# =======================================
FROM alpine:3.16.1

CMD ["/bin/sh"]

WORKDIR /app

COPY --from=builder /app/game-3card-poker ./
ENTRYPOINT ["/app/game-3card-poker", "--config", "/config"]