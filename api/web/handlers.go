package web

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/minishd/minnatropolis/datastore"
)

// How long ago somebody's last visit must be
// in order for us to update it again
const lastVisitUpdateMinInterval = time.Second * 10

// Middleware that catches errors, conditionally logs,
// and sends back an appropriate HTTP response
//
// Also kind of wraps an error-returning handler into a normal [http.Handler],
// which can then be chained with other middlewares
type FallibleHandler func(w http.ResponseWriter, r *http.Request) (err error)

func (handler FallibleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
type SessionHandler func(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error)

// Session handler wrapper for authentication,
// with extra requirements exposed
func requireAuth(ds *datastore.DataStore, next SessionHandler,
	allowBanned bool, minPermLevel datastore.PermissionLevel,
) FallibleHandler {
	return func(w http.ResponseWriter, r *http.Request) (err error) {
		session, err := GetAuth(ds, r)
		if err != nil {
			return
		}
		if session == nil {
			return ErrUnauthorized
		}
		user := session.ForUser

		// If we don't allow banned users here,
		// and the user has an active ban, reject the request
		if !allowBanned && user.HasPunishment(datastore.PunishmentBan) {
			return ErrNoBanned
		}

		// Make sure they have a high enough permission level
		if user.Role.PermissionLevel() < minPermLevel {
			return ErrNoPermission
		}

		// Update their last visit time if it wasn't done
		// already recently
		if time.Since(user.LastVisitAt) >= lastVisitUpdateMinInterval {
			ctx := r.Context()
			if err = ds.UpdateUserLastVisit(ctx, user.ID, time.Now()); err != nil {
				// We could log the error and move on
				// but if the DB isn't working there is probably
				// an issue anyway
				return
			}
		}

		return next(w, r, session)
	}
}

// Session handler wrapper for authentication, requiring moderator permission
func RequireAuthModerator(ds *datastore.DataStore, next SessionHandler) FallibleHandler {
	return requireAuth(ds, next, false, datastore.PermissionLevelModerator)
}

// Session handler wrapper for authentication, requiring moderator permission
func RequireAuthAdministrator(ds *datastore.DataStore, next SessionHandler) FallibleHandler {
	return requireAuth(ds, next, false, datastore.PermissionLevelAdministrator)
}

// Session handler wrapper for authentication, barring banned users
func RequireAuthUnbanned(ds *datastore.DataStore, next SessionHandler) FallibleHandler {
	return requireAuth(ds, next, false, datastore.PermissionLevelNone)
}

// Session handler wrapper for authentication
func RequireAuth(ds *datastore.DataStore, next SessionHandler) FallibleHandler {
	return requireAuth(ds, next, true, datastore.PermissionLevelNone)
}
