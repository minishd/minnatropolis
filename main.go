package main

import (
	"context"
	"embed"
	"encoding/hex"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/minishd/minnatropolis/api"
	"github.com/minishd/minnatropolis/api/room/filters"
	"github.com/minishd/minnatropolis/datastore"
	"github.com/pressly/goose/v3"
)

//go:embed sql/migrations/*.sql
var embedMigrations embed.FS

type configFile struct {
	DBConnect string
	ListenOn  string

	GuardPSK string
	Filter   struct {
		IndexPath       string
		PictureNames    []string
		PicturePrefixes []string
		BattleAnimIDs   []int32
	}
}

func run(rootCtx context.Context) error {
	// Make context for graceful shutdown
	ctx, cancel := signal.NotifyContext(rootCtx, os.Interrupt)
	defer cancel()

	// Load args
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	// Load config
	configData, err := os.ReadFile(*configPath)
	if err != nil {
		return err
	}
	var cfg configFile
	if err := yaml.Unmarshal(configData, &cfg); err != nil {
		return err
	}

	// Check config
	guardPSK, err := hex.DecodeString(cfg.GuardPSK)
	if err != nil {
		return err
	}
	if cfg.DBConnect == "" {
		return errors.New("no db connect URI was specified")
	}
	if cfg.Filter.IndexPath == "" {
		return errors.New("index path wasn't specified")
	}

	// Set up filters
	filters, err := filters.Load(
		cfg.Filter.IndexPath,
		cfg.Filter.BattleAnimIDs,
		cfg.Filter.PictureNames,
		cfg.Filter.PicturePrefixes,
	)
	if err != nil {
		return err
	}

	// Connect to DB
	pool, err := pgxpool.New(ctx, cfg.DBConnect)
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

	// Set up DB wrapper
	ds := datastore.New(pool)

	// Set up API
	mux := http.NewServeMux()
	api.AddRoutes(mux, guardPSK, ds, filters)

	// Set up server
	server := &http.Server{
		Addr:    cfg.ListenOn,
		Handler: mux,
	}

	// Start shutdown watcher
	var wg sync.WaitGroup
	wg.Go(func() {
		<-ctx.Done()

		shutdownCtx, shutdownCancel := context.WithTimeout(rootCtx, 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Println("gracefully shutdown failed, err=", err)
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
