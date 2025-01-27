# ベースイメージ
FROM golang:1.24-alpine AS builder

# 必要なツールをインストール
RUN apk add --no-cache git

# 作業ディレクトリを設定
WORKDIR /app

# Goモジュールファイルとソースコードをコピー
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# アプリケーションをビルド
RUN go build -o server .

# 実行用の軽量イメージを作成
FROM alpine:latest

# 必要なファイルをコピー
WORKDIR /app
COPY --from=builder /app/server .
COPY .env .

# サーバーを起動
CMD ["./server"]
