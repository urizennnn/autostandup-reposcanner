package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"

    "github.com/urizennnn/autostandup-reposcanner/config"
    "github.com/urizennnn/autostandup-reposcanner/redis"
)

var consumerName = fmt.Sprintf("%s-%d", "auto-standup-repo-scanner-1", os.Getpid())

func main() {
	fmt.Println("Starting Repo Scanner Service...")
	cfg, err := config.NewLoader("APP").Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	_ = cfg

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()
    mux := http.NewServeMux()
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
            w.WriteHeader(http.StatusMethodNotAllowed)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{
            "status":   "ok",
            "service":  "repo-scanner",
            "consumer": consumerName,
        })
    })

    httpSrv := &http.Server{Addr: ":8080", Handler: mux}
    go func() {
        log.Printf("http server listening on %s", httpSrv.Addr)
        if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("http server error: %v", err)
        }
    }()

    go func() {
        <-ctx.Done()
        shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownGrace)
        defer cancel()
        if err := httpSrv.Shutdown(shutdownCtx); err != nil {
            log.Printf("http server shutdown error: %v", err)
        }
    }()
	rdb, err := redis.ConnectToRedis(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("Redis connection error: %v", err)
	}
	err = redis.WatchStreams(ctx, rdb, "scan:jobs", "scanners", consumerName)
	if err != nil && ctx.Err() == nil {
		log.Fatalf("WatchStreams %v", err)
	}
}
