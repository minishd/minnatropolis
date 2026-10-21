package web

import (
	"errors"
	"log"
	"net/http"

	"github.com/minishd/minnatropolis/datastore"
)

// Middleware that catches errors, conditionally logs,
// and sends back an appropriate HTTP response
//
// Also kind of wraps an error-returning handler into a normal [http.Handler],
// which can then be chained with other middlewares
type fallibleHandler func(w http.ResponseWriter, r *http.Request) (err error)

func (handler fallibleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Response type that is sent back to the client
	// if their request didn't succeed
	type errorRes struct {
		Error string
	}

	// Call req. handler
	if err := handler(w, r); err != nil {
		// An error was returned
		// See if it's an [Error],
		// werr wrote down status codes and error messages
		// for those that werr want to return
		werr, ok := errors.AsType[*Error](err)
		if !ok {
			// We could not cast it to an [Error]
			// Log it, and fall back to generic "server error" message
			log.Println("handler raised error:", err)
			werr = ErrServerInternal
		}

		// Set status code & send back error response
		if err := SendRes(w, werr.Status, errorRes{werr.Note}); err != nil {
			log.Println("couldn't send error response:", err)
		}
	}
}

// Request handler that additionally receives the
// session of the user that sent the request
type sessionHandler func(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error)

// Session handler wrapper for authentication,
// optionally barring banned users
func requireAuth(ds *datastore.DataStore, allowBanned bool, next sessionHandler) fallibleHandler {
	return func(w http.ResponseWriter, r *http.Request) (err error) {
		session, err := GetAuth(ds, r)
		if err != nil {
			return
		}
		if session == nil {
			return ErrUnauthorized
		}

		// If we don't allow banned users here,
		// and the user has an active ban, reject the request
		if !allowBanned && session.ForUser.HasPunishment(datastore.PunishmentBan) {
			return ErrNoBanned
		}

		return next(w, r, session)
	}
}

// Session handler wrapper for authentication, barring banned users
func RequireAuthUnbanned(ds *datastore.DataStore, next sessionHandler) fallibleHandler {
	return requireAuth(ds, false, next)
}

// Session handler wrapper for authentication
func RequireAuth(ds *datastore.DataStore, next sessionHandler) fallibleHandler {
	return requireAuth(ds, true, next)
}
