package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sigxcpu/tv-dvr208/backend/internal/adapters/inbound/httpapi"
	"github.com/sigxcpu/tv-dvr208/backend/internal/adapters/outbound/ffmpeg"
	"github.com/sigxcpu/tv-dvr208/backend/internal/adapters/outbound/isapi"
	"github.com/sigxcpu/tv-dvr208/backend/internal/application"
	"github.com/sigxcpu/tv-dvr208/backend/internal/webui"
)

func main() {
	// Load optional local .env file (ignored by git/docker) before reading environment variables
	loadDotEnv(".env", "../.env")

	dvrHost := envOr("DVR_HOST", "192.168.1.64")
	dvrHTTPPort := envIntOr("DVR_HTTP_PORT", 80)
	dvrRTSPPort := envIntOr("DVR_RTSP_PORT", 554)
	dvrUser := envOr("DVR_USER", "admin")
	dvrPass := envOr("DVR_PASS", "")
	listenAddr := envOr("LISTEN_ADDR", ":8080")

	if dvrPass == "" {
		log.Printf("WARNING: DVR_PASS is not set (configure via environment variable or .env file)")
	}

	baseURL := fmt.Sprintf("http://%s:%d", dvrHost, dvrHTTPPort)

	// Driven (Outbound) Adapters
	isapiClient := isapi.NewClient(baseURL, dvrUser, dvrPass, 15*time.Second)
	mediaStreamer := ffmpeg.NewStreamer(dvrHost, dvrRTSPPort, dvrUser, dvrPass)

	// Application Core (Use Cases)
	appService := application.NewService(
		isapiClient,
		isapiClient,
		isapiClient,
		isapiClient,
		mediaStreamer,
	)

	// Embedded Web UI (embed.FS)
	distFS, err := webui.DistFS()
	if err != nil {
		log.Fatalf("failed to load embedded web UI: %v", err)
	}

	// Driving (Inbound) HTTP Adapter
	httpHandler := httpapi.NewHandler(appService, distFS)

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           httpHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("TV-DVR208 Hexagonal Gateway listening on %s (upstream DVR: %s)", listenAddr, baseURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func loadDotEnv(paths ...string) {
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			k = strings.TrimSpace(k)
			v = strings.Trim(strings.TrimSpace(v), `"'`)
			if k != "" && os.Getenv(k) == "" {
				_ = os.Setenv(k, v)
			}
		}
		_ = f.Close()
		return
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
