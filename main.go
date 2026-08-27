package main

import (
	"context"
	"embed"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

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

func getFilterList(key string) (list []string) {
	listStr := os.Getenv(key)
	if listStr == "" {
		return
	}
	list = strings.Split(listStr, "|")
	return
}

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

	pictureNames := getFilterList("FILTER_PICTURE_NAMES")
	picturePrefixes := getFilterList("FILTER_PICTURE_PREFIXES")
	battleAnimIDsStr := getFilterList("FILTER_BATTLE_ANIM_IDS")
	var battleAnimIDs []int32
	for _, idStr := range battleAnimIDsStr {
		id_, err := strconv.Atoi(idStr)
		if err != nil {
			return err
		}
		battleAnimIDs = append(battleAnimIDs, int32(id_))
	}

	indexPath := os.Getenv("FILTER_INDEX_PATH")
	filters, err := filters.Load(indexPath, battleAnimIDs, pictureNames, picturePrefixes)
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

	// Set up DB wrapper
	ds := datastore.New(pool)

	// Set up API
	mux := http.NewServeMux()
	api.AddRoutes(mux, guardPSK, ds, filters)

	// Set up server
	server := &http.Server{
		Addr:    listenOn,
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
