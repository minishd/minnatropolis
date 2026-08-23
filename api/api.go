package api

import (
	"net/http"
	"time"

	"github.com/lxzan/gws"
	"github.com/minishd/minnatropolis/api/room"
	"github.com/minishd/minnatropolis/api/web"
	"github.com/minishd/minnatropolis/datastore"
	"golang.org/x/time/rate"
)

const (
	registerRateLimitEvery = 5 * time.Minute
	loginRateLimitEvery    = 20 * time.Second

	registerRateLimitBurst = 5
	loginRateLimitBurst    = 3
)

func AddRoutes(mux *http.ServeMux, guardPSK []byte, ds *datastore.DataStore) {
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
	ah := &authHandlers{ds}

	// Set limits (rate limiting)
	registerLimiter := web.NewLimiter(rate.Every(registerRateLimitEvery), registerRateLimitBurst)
	loginLimiter := web.NewLimiter(rate.Every(loginRateLimitEvery), loginRateLimitBurst)

	authMux.Handle("POST /register", registerLimiter.Check(ah.handleRegister))
	authMux.Handle("POST /login", loginLimiter.Check(ah.handleLogin))
	authMux.Handle("POST /logout", web.RequireAuth(ds, ah.handleLogout))
	authMux.Handle("POST /logout-others", web.RequireAuth(ds, ah.handleLogoutOthers))
	authMux.Handle("POST /renew", web.RequireAuth(ds, ah.handleRenew))
	authMux.Handle("GET /whoami", web.RequireAuth(ds, ah.handleWhoami))

	// Set routes
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
}
