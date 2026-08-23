package web

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/minishd/minnatropolis/datastore"
)

var validate *validator.Validate = validator.New(
	validator.WithRequiredStructEnabled(),
	validator.WithTagNameFuncBlankOmit(),
)

// Reads and looks up a provided session token
func getAuth(ds *datastore.DataStore, r *http.Request) (st *datastore.SessionToken, err error) {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))

	// Make sure prefix is present
	token, found := strings.CutPrefix(authorization, "Session ")
	if !found {
		return
	}

	// Look up token
	ctx := r.Context()
	st, err = ds.LookupSessionToken(ctx, token)
	if err != nil {
		return
	}

	return
}

// Session handler wrapper for authentication
func RequireAuth(ds *datastore.DataStore, next sessionHandler) handleError {
	return func(w http.ResponseWriter, r *http.Request) (err error) {
		session, err := getAuth(ds, r)
		if err != nil {
			return
		}
		if session == nil {
			return ErrUnauthorized
		}

		return next(w, r, session)
	}
}

// Parses the body of a request from JSON
func ParseReq[Req any](r *http.Request) (req Req, err error) {
	// Only proceed if Content-Type is JSON
	// TODO: make this more lenient?
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		err = ErrNotJSON
		return
	}

	// Decode body
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err = dec.Decode(&req); err != nil {
		err = ErrBodyMalformed
		return
	}

	// Validate
	if err = validate.Struct(req); err != nil {
		// Is the struct tag just invalid?
		if _, ok := errors.AsType[*validator.InvalidValidationError](err); ok {
			// `validate` struct tag is malformed.
			// This is our fault and should be fixed
			// before shipping
			panic(err)
		}

		// No the struct tag wasn't invalid,
		// so it is the user's fault
		err = ErrBodyInvalid
		return
	}

	return
}

// Send back a JSON response
func SendRes[Res any](w http.ResponseWriter, status int, res Res) (err error) {
	// Set Content-Type, encode & send
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err = json.NewEncoder(w).Encode(res)
	return
}

// Send back a JSON response with status `200 OK`
func SendResOK[Res any](w http.ResponseWriter, res Res) error {
	return SendRes(w, http.StatusOK, res)
}

// Request handler that additionally receives the
// session of the user that sent the request
type sessionHandler func(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error)

// Middleware that catches errors, conditionally logs,
// and sends back an appropriate HTTP response
//
// Also kind of wraps an error-returning handler into a normal [http.Handler],
// which can then be chained with other middlewares
type handleError func(w http.ResponseWriter, r *http.Request) (err error)

// Response type that is sent back to the client
// if their request didn't succeed
type errorRes struct {
	Error string
}

func (handler handleError) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Call req. handler
	if err := handler(w, r); err != nil {
		// An error was returned
		// See if it's a [WebError],
		// werr wrote down status codes and error messages
		// for those that werr want to return
		werr, ok := errors.AsType[*Error](err)
		if !ok {
			// We could not cast it to a [WebError]
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
