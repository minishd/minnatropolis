package tropolis

import (
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/lxzan/gws"
	"github.com/minishd/minnatropolis/tropolis/protocol"
	"github.com/minishd/minnatropolis/tropolis/room"
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
		ParallelEnabled: true,
		Recovery:        gws.Recovery,
		SubProtocols:    []string{"binary"}, // If unspecified, Chromium instantly disconnects

		Authorize: room.Authorize,
	})

	// Set routes
	http.HandleFunc("/room", func(w http.ResponseWriter, r *http.Request) {
		socket, err := upgrader.Upgrade(w, r)
		if err != nil {
			return
		}
		go socket.ReadLoop()
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello!!!"))
	})

	// Start server
	protocol.RegisterAllPackets()
	slog.Info("starting")
	http.ListenAndServe(listenOn, nil)
}
