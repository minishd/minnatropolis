package main

import (
	"context"
	"embed"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/minishd/minnatropolis/api"
	"github.com/minishd/minnatropolis/db"
	"github.com/pressly/goose/v3"
)

//go:embed sql/migrations/*.sql
var embedMigrations embed.FS

func run(rootCtx context.Context) error {
	// Make context for graceful shutdown
	ctx, cancel := signal.NotifyContext(rootCtx, os.Interrupt)
	defer cancel()

	// Load env
	listenOn := os.Getenv("LISTEN_ON")
	dbConnect := os.Getenv("DB_CONNECT")
	guardPSK, err := hex.DecodeString(os.Getenv("GUARD_PSK"))
	if err != nil {
		return err
	}

	// Connect to DB
	pool, err := pgxpool.New(ctx, dbConnect)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Run migrations
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	stdDB := stdlib.OpenDBFromPool(pool)
	log.Println("running migrations")
	if err := goose.UpContext(ctx, stdDB, "sql/migrations"); err != nil {
		return err
	}
	if err := stdDB.Close(); err != nil {
		return err
	}

	// Test database
	q := db.New(pool)
	q.CreateExample(ctx, db.CreateExampleParams{
		Name:        "hi",
		DisplayName: pgtype.Text{String: "hi world test", Valid: true},
	})

	// Set up API
	mux := http.NewServeMux()
	api.AddRoutes(mux, guardPSK)

	// Set up server
	server := &http.Server{
		Addr:        listenOn,
		Handler:     mux,
		BaseContext: func(l net.Listener) context.Context { return ctx },
	}

	// Start shutdown watcher
	var wg sync.WaitGroup
	wg.Go(func() {
		<-ctx.Done()

		shutdownCtx, shutdownCancel := context.WithTimeout(rootCtx, 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Println("gracefully shutdown failed", "err", err)
			return
		}
	})

	// Start server
	log.Println("starting")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	wg.Wait()
	log.Println("stopped")

	return nil
}

func main() {
	godotenv.Load()

	ctx := context.Background()
	if err := run(ctx); err != nil {
		log.Fatal("initialization failed: ", err)
	}
}
