package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/lxzan/gws"
	"github.com/minishd/minnatropolis/api/room"
	"github.com/minishd/minnatropolis/api/weberrors"
	"github.com/minishd/minnatropolis/datastore"
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
	authMux.Handle("POST /register", handleError(ah.handleRegister))
	authMux.Handle("POST /login", handleError(ah.handleLogin))
	authMux.Handle("GET /whoami", http.HandlerFunc(ah.handleWhoami))

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

// Response type that is sent back to the client
// if their request didn't succeed
type errorResponse struct {
	Error string
}

var validate *validator.Validate = validator.New(
	validator.WithRequiredStructEnabled(),
	validator.WithTagNameFuncBlankOmit(),
)

// Parses the body of a request from JSON
func parseReq[Req any](r *http.Request) (req Req, err error) {
	// Only proceed if Content-Type is JSON
	// TODO: make this more lenient?
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		err = weberrors.ErrNotJSON
		return
	}

	// Decode body
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err = dec.Decode(&req); err != nil {
		err = weberrors.ErrBodyMalformed
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
		err = weberrors.ErrBodyInvalid
		return
	}

	return
}

// Send back a JSON response
func sendRes[Res any](w http.ResponseWriter, res Res) (err error) {
	// Set Content-Type, encode & send
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(res)
	return
}

// Middleware that catches errors, conditionally logs,
// and sends back an appropriate HTTP response
type handleError func(w http.ResponseWriter, r *http.Request) (err error)

func (handler handleError) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Call req. handler
	if err := handler(w, r); err != nil {
		// An error was returned
		// See if it's a [weberrors.WebError],
		// werr wrote down status codes and error messages
		// for those that werr want to return
		werr, ok := errors.AsType[*weberrors.WebError](err)
		if !ok {
			// We could not cast it to a [weberrors.WebError]
			// Log it, and fall back to generic "server error" message
			log.Println("handler raised error:", err)
			werr = weberrors.ErrServerInternal
		}

		// Set status code & send back error response
		w.WriteHeader(werr.Status)
		if err := sendRes(w, errorResponse{werr.Note}); err != nil {
			log.Println("couldn't send error response:", err)
		}
	}
}
