package api

import (
	"context"
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
	authMux.Handle("POST /register", wrapWithReq(ah.handleRegister))
	authMux.Handle("POST /login", wrapWithReq(ah.handleLogin))
	authMux.Handle("GET /whoami", wrap(ah.handleWhoami))

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

func wrap[Res any](handler func(context.Context) (Res, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Call req. handler
		res, err := handler(r.Context())
		var finalRes any = res
		if err != nil {
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
			// Change the final response to an error response,
			// and set the status code
			finalRes = errorResponse{werr.Note}
			w.WriteHeader(werr.Status)
		}

		// Set response content-type header to JSON
		// and send JSON-encoded response struct
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(finalRes); err != nil {
			// We shouldn't be failing JSON serializations
			// That is something that can be accounted for
			// at compile-time for the most part
			panic(err)
		}

	})
}

func wrapWithReq[Req any, Res any](handler func(context.Context, Req) (Res, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Make sure content-type is JSON
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}

		// Decode request body from JSON
		var req Req
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Validate
		if err := validate.Struct(req); err != nil {
			// Is the struct tag just invalid?
			if _, ok := errors.AsType[*validator.InvalidValidationError](err); ok {
				// `validate` struct tag is malformed.
				// This is our fault and should be fixed
				// before shipping
				panic(err)
			}

			// No the struct tag wasn't invalid,
			// so it is the user's fault
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}

		// Call req. handler with decoded and
		// validated request struct
		res, err := handler(r.Context(), req)
		var final any = res
		if err != nil {
			we, ok := errors.AsType[*weberrors.WebError](err)
			if !ok {
				// It's not an expected error
				log.Println("handler raised error:", err)
				we = weberrors.ErrServerInternal
			}
			final = errorResponse{we.Note}
			w.WriteHeader(we.Status)
		}

		// Set response content-type header to JSON
		// and send JSON-encoded response struct
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(final); err != nil {
			// We shouldn't be failing JSON serializations
			// That is something that can be accounted for
			// at compile-time for the most part
			panic(err)
		}

	})
}
