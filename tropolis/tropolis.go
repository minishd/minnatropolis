package tropolis

import (
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/lxzan/gws"
	"github.com/minishd/minnatropolis/tropolis/api"
	"github.com/minishd/minnatropolis/tropolis/api/weberrors"
	"github.com/minishd/minnatropolis/tropolis/room"
	"github.com/minishd/minnatropolis/tropolis/room/protocol"
)

func Start() {
	godotenv.Load()

	// Load env
	listenOn := os.Getenv("LISTEN_ON")
	guardPSK, err := hex.DecodeString(os.Getenv("GUARD_PSK"))
	if err != nil {
		panic(err)
	}

	// Set up upgrader
	rh := room.NewHandler(guardPSK)
	upgrader := gws.NewUpgrader(rh, &gws.ServerOption{
		// Don't process each connection's messages in parallel
		// If we do, the guard message counter check will start
		// dropping legitimate messages that arrived in order,
		// but just didn't get processed in order
		ParallelEnabled: false,
		Recovery:        gws.Recovery,
		SubProtocols:    []string{"binary"}, // If unspecified, Chromium instantly disconnects

		Authorize: room.Authorize,
	})

	// Set routes (auth)
	authMux := http.NewServeMux()
	authMux.Handle("POST /test", api.Wrap(handleTest))

	// Set routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /room", func(w http.ResponseWriter, r *http.Request) {
		socket, err := upgrader.Upgrade(w, r)
		if err != nil {
			return
		}
		go socket.ReadLoop()
	})
	mux.Handle("/auth/", http.StripPrefix("/auth", authMux))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("api unconscious"))
	})

	// Start server
	protocol.RegisterAllPackets()
	slog.Info("starting")
	http.ListenAndServe(listenOn, mux)
}

type loginRequest struct {
	Username string
	Password string
}

type loginResponse struct {
	Token string
}

var example = map[string]string{
	"hi":   "world",
	"some": "other",
}

func handleTest(req loginRequest) (res loginResponse, err error) {
	pw, ok := example[req.Username]
	if !ok {
		err = weberrors.ErrUserNotFound
		return
	}
	if req.Password == "evil password" {
		err = errors.New("awful terrible error")
		return
	}
	if pw != req.Password {
		err = weberrors.ErrInvalidCredentials
		return
	}
	res.Token = "hi world"
	return
}
