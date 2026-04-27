package tropolis

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"

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
		em: ee.New[int32, *Sub](&ee.Config{
			BucketNum:  128,
			BucketSize: 128,
		}),
	}
	upgrader := gws.NewUpgrader(rh, &gws.ServerOption{
		ParallelEnabled: true,
		Recovery:        gws.Recovery,
		SubProtocols:    []string{"binary"}, // If unspecified, Chromium instantly disconnects

		// In the future, this should check tokens probably
		// (Either here or in [roomHandler.OnOpen])
		Authorize: func(r *http.Request, session gws.SessionStorage) bool {
			// Get room ID
			roomID, err := strconv.Atoi(r.URL.Query().Get("id"))
			if err != nil {
				return false
			}
			if roomID < 1 || roomID > 5000 {
				return false
			}

			// Get token
			token := r.URL.Query().Get("token")

			// Set up data
			session.Store("cd", clientData{
				cID:      cIDCounter.Add(1),
				name:     token, // Temporary
				guardKey: randomString(12),
				mapID:    int32(roomID),
			})

			// Authorize connection
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
	registerAllPackets()
	slog.Info("starting")
	http.ListenAndServe(listenOn, nil)
}
