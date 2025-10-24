package main

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"image-service/internal/cache"
	"image-service/internal/handler"
	"image-service/internal/middleware"
	"image-service/internal/processor"
	"image-service/pkg/config"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	cfg := config.Load()

	log.Println("🚀 Starting Image Transformation Service...")
	log.Printf("📝 Port: %s", cfg.Port)
	log.Printf("📝 Allowed domains: %v", cfg.AllowedDomains)
	log.Printf("📝 Redis: %s", cfg.RedisURL)

	cacheClient, err := cache.NewCache(cfg.RedisURL, cfg.RedisPassword, cfg.CacheTTL)
	if err != nil {
		log.Fatalf("❌ Failed to connect to Redis: %v", err)
	}
	defer cacheClient.Close()
	log.Println("✅ Redis connected")

	proc := processor.NewProcessor()
	defer proc.Shutdown()
	log.Println("✅ Image processor initialized")

	h := handler.NewHandler(cacheClient, proc, cfg.MaxImageSize)

	r := mux.NewRouter()

	rateLimiter := middleware.NewRateLimiter(cfg.RateLimit)

	r.HandleFunc("/health", h.Health).Methods("GET")
	r.Handle("/transform",
		rateLimiter.Limit(
			middleware.Auth(cfg.AllowedDomains)(
				http.HandlerFunc(h.Transform),
			),
		),
	).Methods("GET")

	r.Use(corsMiddleware)
	r.Use(compressionMiddleware)

	addr := fmt.Sprintf(":%s", cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Println("\n🛑 Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("❌ Server shutdown error: %v", err)
		}

		log.Println("✅ Server stopped gracefully")
	}()

	log.Printf("🌐 Server listening on http://localhost%s", addr)
	log.Println("✨ Ready to transform images!")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ Server error: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func compressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/transform") {
			next.ServeHTTP(w, r)
			return
		}

		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()

		gzw := gzipResponseWriter{Writer: gz, ResponseWriter: w}
		next.ServeHTTP(gzw, r)
	})
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}
