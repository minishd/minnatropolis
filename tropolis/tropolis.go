package tropolis

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	ee "github.com/lxzan/event_emitter"
	"github.com/lxzan/gws"
)

func Start() {
	godotenv.Load()

	// Load env
	listenOn := os.Getenv("LISTEN_ON")

	// Set up upgrader
	rh := &roomHandler{
		em: ee.New[uint64, *Sub](&ee.Config{
			BucketNum:  128,
			BucketSize: 128,
		}),
	}
	upgrader := gws.NewUpgrader(rh, &gws.ServerOption{
		ParallelEnabled: true,
		Recovery:        gws.Recovery,

		// In the future, this should check tokens probably
		// (Either here or in [roomHandler.OnOpen])
		Authorize: func(r *http.Request, session gws.SessionStorage) bool {
			slog.Info("authorizing connection", slog.Any("roomID", r.URL.Query().Get("id")))

			// Authorize every connection
			return true
		},
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
	slog.Info("starting")
	http.ListenAndServe(listenOn, nil)
}
