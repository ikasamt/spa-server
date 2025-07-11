package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

func getClientIP(r *http.Request) string {
	// X-Forwarded-For ヘッダーをチェック
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		// 最初のIPアドレスを取得（クライアントに最も近いIP）
		return strings.TrimSpace(ips[0])
	}
	// フォールバックとしてRemoteAddrを使用
	log.Println(r.RemoteAddr)
	return strings.Split(r.RemoteAddr, ":")[0]
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// .env ファイルを読み込み
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	// 環境変数の取得
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // デフォルトポート
	}

	distDir := os.Getenv("DIST_DIR")
	if distDir == "" {
		fmt.Println("Error: DIST_DIR is not defined in .env")
		os.Exit(1)
	}

	allowRemoteIPs := os.Getenv("ALLOW_REMOTE_IPS")
	allowedIPs := strings.Split(allowRemoteIPs, ",")
	log.Println(allowRemoteIPs)

	// 指定されたディレクトリが存在するか確認
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		fmt.Printf("Error: Directory %s does not exist.\n", distDir)
		os.Exit(1)
	}

	// ファイルサーバーを作成
	fileServer := http.FileServer(http.Dir(distDir))

	// リクエストハンドラ
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// クライアントIPアドレスを取得
		clientIP := getClientIP(r)

		// 許可されたIPの確認
		if len(allowedIPs) > 0 && allowedIPs[0] != "" { // 設定がある場合
			allowed := false
			for _, allowedIP := range allowedIPs {
				// 完全一致または前方一致をチェック
				if allowedIP == clientIP || strings.HasPrefix(clientIP, strings.TrimSpace(allowedIP)) {
					allowed = true
					break
				}
			}
			if !allowed {
				// ログ出力
				log.Println("Client IP: ", clientIP)
				log.Println("X-Forwarded-For: ", r.Header.Get("X-Forwarded-For"))
				log.Println("RemoteAddr: ", r.RemoteAddr)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}

		// ファイルパスを確認
		filePath := filepath.Join(distDir, r.URL.Path)

		// ファイルが存在しない場合は index.html を返す
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // index.html にはキャッシュさせない
			http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
		} else {
			// 静的ファイルを提供
			if r.URL.Path == "/" || r.URL.Path == "/index.html" {
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // index.html にはキャッシュさせない
			}
			fileServer.ServeHTTP(w, r)
		}
	})

	// サーバー起動
	log.Println("Serving on http://localhost:", port)
	http.ListenAndServe(":"+port, nil)
}
