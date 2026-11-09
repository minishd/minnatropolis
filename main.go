package main

import (
	"context"
	"embed"
	"encoding/hex"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/minishd/minnatropolis/api"
	"github.com/minishd/minnatropolis/api/chat"
	"github.com/minishd/minnatropolis/api/room"
	"github.com/minishd/minnatropolis/api/room/filters"
	"github.com/minishd/minnatropolis/api/room/unconscious"
	"github.com/minishd/minnatropolis/datastore"
	"github.com/pressly/goose/v3"
)

//go:embed sql/migrations/*.sql
var embedMigrations embed.FS

func run(rootCtx context.Context) error {
	// Make context for graceful shutdown
	ctx, cancel := signal.NotifyContext(rootCtx, os.Interrupt)
	defer cancel()

	// Load args
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	// Load config
	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}

	// Parse guard PSK
	guardPSK, err := hex.DecodeString(cfg.GuardPSK)
	if err != nil {
		return err
	}

	// Set up filters
	filters, err := filters.Load(
		cfg.Filter.IndexPath,
		cfg.Filter.BattleAnimIDs,
		cfg.Filter.PictureNames,
		cfg.Filter.PicturePrefixes,
		cfg.Filter.SingleplayerRooms,
	)
	if err != nil {
		return err
	}

	// Set up text filters
	tfChat, err := initTextFilter(cfg.TextFilter.Chat)
	if err != nil {
		return err
	}
	tfUsernames, err := initTextFilter(cfg.TextFilter.Usernames)
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

	// Set up Collective Unconscious state
	var coun *unconscious.Unconscious
	if cfg.Unconscious {
		coun = unconscious.New()
	}

	// Set up API
	mux := http.NewServeMux()
	rh := room.NewHandler(ds, guardPSK, filters, coun)
	ch := chat.NewHandler(ds, rh, tfChat)
	api.AddRoutes(mux, rh, ch, guardPSK, ds, filters, coun, tfChat, tfUsernames)

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

	// Start room handler background
	wg.Go(func() { rh.Background(ctx) })

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
	ctx := context.Background()
	if err := run(ctx); err != nil {
		log.Fatal("initialization failed: ", err)
	}
}
