package api

import (
	"net/http"
	"time"

	"github.com/lxzan/gws"
	"github.com/minishd/minnatropolis/api/chat"
	"github.com/minishd/minnatropolis/api/room"
	"github.com/minishd/minnatropolis/api/room/filters"
	"github.com/minishd/minnatropolis/api/web"
	"github.com/minishd/minnatropolis/datastore"
	"golang.org/x/time/rate"
)

const (
	registerRateLimitEvery = 5 * time.Minute
	loginRateLimitEvery    = 20 * time.Second
	roomRateLimitEvery     = 5 * time.Second
	chatRateLimitEvery     = 5 * time.Second

	registerRateLimitBurst = 5
	loginRateLimitBurst    = 3
	roomRateLimitBurst     = 30
	chatRateLimitBurst     = 30

	chatMaxMessageSize = 4 * 1024
)

func AddRoutes(mux *http.ServeMux, guardPSK []byte, ds *datastore.DataStore, filters *filters.Filters, unconscious bool) {
	// Set up upgrader
	rh := room.NewHandler(ds, guardPSK, filters, unconscious)
	upgrader := gws.NewUpgrader(rh, &gws.ServerOption{
		// Don't process each connection's messages in parallel
		// If we do, the guard message counter check will start
		// dropping legitimate messages that arrived in order,
		// but just didn't get processed in order
		ParallelEnabled: false,
		Recovery:        gws.Recovery,
		SubProtocols:    []string{"binary"}, // If unspecified, Chromium instantly disconnects

		Authorize: rh.Authorize,
	})

	// Set up chat upgrader
	ch := chat.NewHandler(ds, rh)
	chatUpgrader := gws.NewUpgrader(ch, &gws.ServerOption{
		ParallelEnabled:    false,
		Recovery:           gws.Recovery,
		ReadMaxPayloadSize: chatMaxMessageSize,

		Authorize: ch.Authorize,
	})

	// Set routes (auth)
	authMux := http.NewServeMux()
	ah := &authHandlers{ds}

	// Set limits (rate limiting)
	registerLimiter := web.NewLimiter(rate.Every(registerRateLimitEvery), registerRateLimitBurst)
	loginLimiter := web.NewLimiter(rate.Every(loginRateLimitEvery), loginRateLimitBurst)
	roomLimiter := web.NewLimiter(rate.Every(roomRateLimitEvery), roomRateLimitBurst)
	chatLimiter := web.NewLimiter(rate.Every(chatRateLimitEvery), chatRateLimitBurst)

	authMux.Handle("POST /register", registerLimiter.Check(ah.handleRegister))
	authMux.Handle("POST /login", loginLimiter.Check(ah.handleLogin))
	authMux.Handle("POST /logout", web.RequireAuth(ds, ah.handleLogout))
	authMux.Handle("POST /logout-others", web.RequireAuth(ds, ah.handleLogoutOthers))
	authMux.Handle("POST /renew", web.RequireAuth(ds, ah.handleRenew))

	// Set routes (users)
	usersMux := http.NewServeMux()
	uh := &usersHandlers{ds, rh, ch}
	usersMux.Handle("GET /me", web.RequireAuth(ds, uh.handleMe))
	usersMux.Handle("GET /blocklist", web.RequireAuth(ds, uh.handleBlockList))
	usersMux.Handle("POST /blocklist", web.RequireAuth(ds, uh.handleBlockListAdd))
	usersMux.Handle("DELETE /blocklist", web.RequireAuth(ds, uh.handleBlockListRemove))

	// Set routes (parties)
	partiesMux := http.NewServeMux()
	ph := &partiesHandlers{ds, ch}
	partiesMux.Handle("GET /me", web.RequireAuth(ds, ph.handleMe))
	partiesMux.Handle("POST /create", web.RequireAuth(ds, ph.handleCreate))
	partiesMux.Handle("POST /join", web.RequireAuth(ds, ph.handleJoin))
	partiesMux.Handle("POST /leave", web.RequireAuth(ds, ph.handleLeave))

	// Set routes
	mux.Handle("GET /room", roomLimiter.Check(func(w http.ResponseWriter, r *http.Request) error {
		socket, err := upgrader.Upgrade(w, r)
		if err != nil {
			return nil
		}
		go socket.ReadLoop()
		return nil
	}))
	mux.Handle("GET /chat", chatLimiter.Check(func(w http.ResponseWriter, r *http.Request) error {
		socket, err := chatUpgrader.Upgrade(w, r)
		if err != nil {
			return nil
		}
		go socket.ReadLoop()
		return nil
	}))
	mux.Handle("/auth/", http.StripPrefix("/auth", authMux))
	mux.Handle("/users/", http.StripPrefix("/users", usersMux))
	mux.Handle("/parties/", http.StripPrefix("/parties", partiesMux))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("api unconscious"))
	})
}
