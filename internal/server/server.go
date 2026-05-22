package server

import (
	"brs/internal/auth"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"
)

//go:embed static/*.html
var staticFiles embed.FS

var httpServer *http.Server
var templates *template.Template

type Config struct {
	Host string
	Port string
}

func Init(cfg Config) {
	// Загружаем шаблоны
	loadTemplates()

	httpServer = &http.Server{
		Addr:         cfg.Host + ":" + cfg.Port,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	setupRoutes()
}

func loadTemplates() {
	var err error
	templates, err = template.ParseFS(staticFiles, "static/*.html")
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}
}

// Start запускает HTTP сервер
func Start() error {
	if httpServer == nil {
		return fmt.Errorf("server not initialized. Call Init() first")
	}

	log.Printf("Server starting on http://%s", httpServer.Addr)
	return httpServer.ListenAndServe()
}

// Shutdown gracefully останавливает сервер
func Shutdown(ctx context.Context) error {
	if httpServer == nil {
		return nil
	}

	log.Println("Server shutting down...")
	return httpServer.Shutdown(ctx)
}

var mtlsServer *http.Server

type MTLSConfig struct {
	Enabled  bool
	Port     string
	CAFile   string
	CertFile string
	KeyFile  string
}

func InitMTLSServer(cfg MTLSConfig) error {
	if !cfg.Enabled {
		return nil
	}

	tlsConfig, err := auth.LoadServerTLSConfig(cfg.CAFile, cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to load TLS config: %w", err)
	}

	mtlsServer = &http.Server{
		Addr:      ":" + cfg.Port,
		TLSConfig: tlsConfig,
	}

	// Настройка маршрутов для mTLS сервера
	mtlsMux := http.NewServeMux()
	mtlsMux.HandleFunc("GET /api/mtls/health", mtlsHealthHandler)
	mtlsMux.HandleFunc("GET /api/mtls/protected", mtlsProtectedHandler)
	mtlsServer.Handler = mtlsMux

	log.Printf("mTLS server starting on port %s", cfg.Port)
	return nil
}

func StartMTLSServer() error {
	if mtlsServer == nil {
		return nil
	}
	return mtlsServer.ListenAndServeTLS("", "")
}

func mtlsHealthHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем сертификат клиента
	if r.TLS == nil {
		http.Error(w, "TLS required", http.StatusUnauthorized)
		return
	}

	clientCN := auth.GetClientCN(r.TLS)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"client_cn": clientCN,
		"mtls":      true,
	})
}

func mtlsProtectedHandler(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil {
		http.Error(w, "TLS required", http.StatusUnauthorized)
		return
	}

	clientCN := auth.GetClientCN(r.TLS)
	if clientCN == "" {
		http.Error(w, "No client certificate", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Access granted via mTLS",
		"client":  clientCN,
	})
}
