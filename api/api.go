package api

import (
	"net/http"
	"time"

	"github.com/lxzan/gws"
	"github.com/minishd/minnatropolis/api/auth"
	"github.com/minishd/minnatropolis/api/chat"
	"github.com/minishd/minnatropolis/api/parties"
	"github.com/minishd/minnatropolis/api/room"
	"github.com/minishd/minnatropolis/api/room/filters"
	"github.com/minishd/minnatropolis/api/room/unconscious"
	"github.com/minishd/minnatropolis/api/users"
	"github.com/minishd/minnatropolis/api/web"
	"github.com/minishd/minnatropolis/datastore"
	"github.com/minishd/minnatropolis/textfilter"
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

func AddRoutes(
	mux *http.ServeMux, rh *room.Handler, ch *chat.Handler,
	guardPSK []byte, ds *datastore.DataStore,
	filters *filters.Filters, coun *unconscious.Unconscious,
	tfChat *textfilter.TextFilter, tfUsernames *textfilter.TextFilter,
) {
	// Set up room upgrader
	roomUpgrader := gws.NewUpgrader(rh, &gws.ServerOption{
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
	chatUpgrader := gws.NewUpgrader(ch, &gws.ServerOption{
		ParallelEnabled:    false,
		Recovery:           gws.Recovery,
		ReadMaxPayloadSize: chatMaxMessageSize,

		Authorize: ch.Authorize,
	})

	// Set routes (auth)
	authMux := http.NewServeMux()
	ah := auth.NewHandlers(ds, tfUsernames)

	// Set limits (rate limiting)
	registerLimiter := web.NewLimiter(rate.Every(registerRateLimitEvery), registerRateLimitBurst)
	loginLimiter := web.NewLimiter(rate.Every(loginRateLimitEvery), loginRateLimitBurst)
	roomLimiter := web.NewLimiter(rate.Every(roomRateLimitEvery), roomRateLimitBurst)
	chatLimiter := web.NewLimiter(rate.Every(chatRateLimitEvery), chatRateLimitBurst)

	authMux.Handle("POST /register", registerLimiter.Check(ah.HandleRegister))
	authMux.Handle("POST /login", loginLimiter.Check(ah.HandleLogin))
	authMux.Handle("POST /logout", web.RequireAuth(ds, ah.HandleLogout))
	authMux.Handle("POST /logout-others", web.RequireAuth(ds, ah.HandleLogoutOthers))
	authMux.Handle("POST /renew", web.RequireAuth(ds, ah.HandleRenew))

	// Set routes (users)
	usersMux := http.NewServeMux()
	uh := users.NewHandlers(ds, rh, ch)
	usersMux.Handle("GET /me", web.RequireAuth(ds, uh.HandleMe))
	usersMux.Handle("GET /blocklist", web.RequireAuth(ds, uh.HandleBlockList))
	usersMux.Handle("POST /blocklist", web.RequireAuth(ds, uh.HandleBlockListAdd))
	usersMux.Handle("DELETE /blocklist", web.RequireAuth(ds, uh.HandleBlockListRemove))

	// Set routes (parties)
	partiesMux := http.NewServeMux()
	ph := parties.NewHandlers(ds, ch)
	partiesMux.Handle("GET /me", web.RequireAuth(ds, ph.HandleMe))
	partiesMux.Handle("POST /create", web.RequireAuth(ds, ph.HandleCreate))
	partiesMux.Handle("POST /join", web.RequireAuth(ds, ph.HandleJoin))
	partiesMux.Handle("POST /leave", web.RequireAuth(ds, ph.HandleLeave))

	// Set routes
	mux.Handle("GET /room", roomLimiter.Check(func(w http.ResponseWriter, r *http.Request) error {
		socket, err := roomUpgrader.Upgrade(w, r)
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
