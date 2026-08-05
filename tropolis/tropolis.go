package tropolis

import (
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/minishd/minnatropolis/tropolis/api"
)

func Start() {
	godotenv.Load()

	// Load env
	listenOn := os.Getenv("LISTEN_ON")
	guardPSK, err := hex.DecodeString(os.Getenv("GUARD_PSK"))
	if err != nil {
		panic(err)
	}

	// Set up API
	mux := http.NewServeMux()
	api.AddRoutes(mux, guardPSK)

	// Start server
	slog.Info("starting")
	http.ListenAndServe(listenOn, mux)
}
