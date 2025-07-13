package main

import (
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createHandler(distDir string) http.Handler {
	mux := http.NewServeMux()
	fileServer := http.FileServer(http.Dir(distDir))
	proxyURL := os.Getenv("PROXY_URL")

	// プロキシの設定
	var proxy *httputil.ReverseProxy
	if proxyURL != "" {
		target, err := url.Parse(proxyURL)
		if err != nil {
			log.Printf("Error parsing proxy URL: %v\n", err)
		} else {
			proxy = httputil.NewSingleHostReverseProxy(target)
			proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("Proxy error: %v\n", err)
				http.Error(w, "Bad Gateway", http.StatusBadGateway)
			}
		}
	}

	// プロキシパスの設定
	proxyPaths := os.Getenv("PROXY_PATHS")
	var paths []string
	if proxyPaths != "" {
		paths = strings.Split(proxyPaths, ",")
		for i := range paths {
			paths[i] = strings.TrimSpace(paths[i])
		}
	} else {
		// デフォルトは/query
		paths = []string{"/query"}
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// プロキシパスのチェック
		shouldProxy := false
		for _, path := range paths {
			if strings.HasPrefix(r.URL.Path, path) {
				shouldProxy = true
				break
			}
		}

		if shouldProxy {
			if proxy != nil {
				log.Printf("Proxying request: %s %s\n", r.Method, r.URL.Path)
				proxy.ServeHTTP(w, r)
			} else {
				http.NotFound(w, r)
			}
			return
		}

		// 既存のSPA処理
		filePath := filepath.Join(distDir, r.URL.Path)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
		} else {
			if r.URL.Path == "/" || r.URL.Path == "/index.html" {
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			}
			fileServer.ServeHTTP(w, r)
		}
	})

	return mux
}


func TestProxyEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		proxyURL       string
		requestPath    string
		expectedStatus int
		expectProxy    bool
	}{
		{
			name:           "プロキシURLが設定されている場合、/queryパスへのリクエストはプロキシされる",
			proxyURL:       "http://backend-server:8081",
			requestPath:    "/query",
			expectedStatus: http.StatusOK,
			expectProxy:    true,
		},
		{
			name:           "プロキシURLが設定されている場合、/query/subpathへのリクエストもプロキシされる",
			proxyURL:       "http://backend-server:8081",
			requestPath:    "/query/subpath",
			expectedStatus: http.StatusOK,
			expectProxy:    true,
		},
		{
			name:           "プロキシURLが設定されていない場合、/queryパスへのリクエストは404を返す",
			proxyURL:       "",
			requestPath:    "/query",
			expectedStatus: http.StatusNotFound,
			expectProxy:    false,
		},
		{
			name:           "他のパスへのリクエストはプロキシされない",
			proxyURL:       "http://backend-server:8081",
			requestPath:    "/other",
			expectedStatus: http.StatusOK,
			expectProxy:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 環境変数の設定
			if tt.proxyURL != "" {
				os.Setenv("PROXY_URL", tt.proxyURL)
			} else {
				os.Unsetenv("PROXY_URL")
			}

			// テスト用のバックエンドサーバーを作成
			backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("backend response"))
			}))
			defer backendServer.Close()

			// プロキシURLをテストサーバーのURLに置き換え
			if tt.proxyURL != "" {
				os.Setenv("PROXY_URL", backendServer.URL)
			}

			// テスト用のディレクトリを作成
			tempDir := t.TempDir()
			os.WriteFile(tempDir+"/index.html", []byte("<!DOCTYPE html><html><body>SPA</body></html>"), 0644)
			os.Setenv("DIST_DIR", tempDir)

			// リクエストとレスポンスレコーダーを作成
			req := httptest.NewRequest("GET", tt.requestPath, nil)
			rr := httptest.NewRecorder()

			// ハンドラーを作成して実行
			handler := createHandler(tempDir)
			handler.ServeHTTP(rr, req)

			// ステータスコードの確認
			if rr.Code != tt.expectedStatus {
				t.Errorf("期待されるステータスコード %d, 実際のステータスコード %d", tt.expectedStatus, rr.Code)
			}

			// プロキシの動作確認
			if tt.expectProxy && tt.proxyURL != "" {
				body := rr.Body.String()
				if body != "backend response" {
					t.Errorf("プロキシレスポンスが期待される値と異なります。期待値: 'backend response', 実際: '%s'", body)
				}
			}
		})
	}
}

func TestProxyHeaders(t *testing.T) {
	// テスト用のバックエンドサーバーを作成
	var receivedHeaders http.Header
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer backendServer.Close()

	// 環境変数の設定
	os.Setenv("PROXY_URL", backendServer.URL)
	tempDir := t.TempDir()
	os.WriteFile(tempDir+"/index.html", []byte("<!DOCTYPE html><html><body>SPA</body></html>"), 0644)
	os.Setenv("DIST_DIR", tempDir)

	// リクエストを作成
	req := httptest.NewRequest("GET", "/query", nil)
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom-Header", "custom-value")

	rr := httptest.NewRecorder()

	// ハンドラーを作成して実行
	handler := createHandler(tempDir)
	handler.ServeHTTP(rr, req)

	// ヘッダーが転送されているか確認
	if receivedHeaders.Get("Authorization") != "Bearer token123" {
		t.Error("Authorizationヘッダーが転送されていません")
	}
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Error("Content-Typeヘッダーが転送されていません")
	}
	if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
		t.Error("カスタムヘッダーが転送されていません")
	}
}

func TestProxyMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run("HTTPメソッド_"+method, func(t *testing.T) {
			// テスト用のバックエンドサーバーを作成
			var receivedMethod string
			backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedMethod = r.Method
				w.WriteHeader(http.StatusOK)
			}))
			defer backendServer.Close()

			// 環境変数の設定
			os.Setenv("PROXY_URL", backendServer.URL)
			tempDir := t.TempDir()
			os.WriteFile(tempDir+"/index.html", []byte("<!DOCTYPE html><html><body>SPA</body></html>"), 0644)
			os.Setenv("DIST_DIR", tempDir)

			// リクエストを作成
			req := httptest.NewRequest(method, "/query", nil)
			rr := httptest.NewRecorder()

			// ハンドラーを作成して実行
			handler := createHandler(tempDir)
			handler.ServeHTTP(rr, req)

			// メソッドが正しく転送されているか確認
			if receivedMethod != method {
				t.Errorf("HTTPメソッドが正しく転送されていません。期待値: %s, 実際: %s", method, receivedMethod)
			}
		})
	}
}

func TestMultipleProxyPaths(t *testing.T) {
	tests := []struct {
		name           string
		proxyPaths     string
		requestPath    string
		expectedStatus int
		expectProxy    bool
	}{
		{
			name:           "PROXY_PATHSで/apiパスが設定されている場合、プロキシされる",
			proxyPaths:     "/api",
			requestPath:    "/api/users",
			expectedStatus: http.StatusOK,
			expectProxy:    true,
		},
		{
			name:           "複数のパスが設定されている場合、/queryもプロキシされる",
			proxyPaths:     "/api,/query,/graphql",
			requestPath:    "/query/data",
			expectedStatus: http.StatusOK,
			expectProxy:    true,
		},
		{
			name:           "複数のパスが設定されている場合、/graphqlもプロキシされる",
			proxyPaths:     "/api,/query,/graphql",
			requestPath:    "/graphql",
			expectedStatus: http.StatusOK,
			expectProxy:    true,
		},
		{
			name:           "設定されていないパスはプロキシされない",
			proxyPaths:     "/api,/query",
			requestPath:    "/other",
			expectedStatus: http.StatusOK,
			expectProxy:    false,
		},
		{
			name:           "PROXY_PATHSが空の場合、デフォルトで/queryがプロキシされる",
			proxyPaths:     "",
			requestPath:    "/query",
			expectedStatus: http.StatusOK,
			expectProxy:    true,
		},
		{
			name:           "PROXY_PATHSが空の場合、/api はプロキシされない",
			proxyPaths:     "",
			requestPath:    "/api",
			expectedStatus: http.StatusOK,
			expectProxy:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// テスト用のバックエンドサーバーを作成
			backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("backend response"))
			}))
			defer backendServer.Close()

			// 環境変数の設定
			os.Setenv("PROXY_URL", backendServer.URL)
			if tt.proxyPaths != "" {
				os.Setenv("PROXY_PATHS", tt.proxyPaths)
			} else {
				os.Unsetenv("PROXY_PATHS")
			}

			// テスト用のディレクトリを作成
			tempDir := t.TempDir()
			os.WriteFile(tempDir+"/index.html", []byte("<!DOCTYPE html><html><body>SPA</body></html>"), 0644)
			os.Setenv("DIST_DIR", tempDir)

			// リクエストとレスポンスレコーダーを作成
			req := httptest.NewRequest("GET", tt.requestPath, nil)
			rr := httptest.NewRecorder()

			// ハンドラーを作成して実行
			handler := createHandler(tempDir)
			handler.ServeHTTP(rr, req)

			// ステータスコードの確認
			if rr.Code != tt.expectedStatus {
				t.Errorf("期待されるステータスコード %d, 実際のステータスコード %d", tt.expectedStatus, rr.Code)
			}

			// プロキシの動作確認
			if tt.expectProxy {
				body := rr.Body.String()
				if body != "backend response" {
					t.Errorf("プロキシレスポンスが期待される値と異なります。期待値: 'backend response', 実際: '%s'", body)
				}
			}
		})
	}
}