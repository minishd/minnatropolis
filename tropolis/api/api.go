package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/minishd/minnatropolis/tropolis/api/weberrors"
)

type errorResponse struct {
	Error string
}

func Wrap[Req any, Res any](handler func(Req) (Res, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}

		var req Req
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		res, err := handler(req)
		var final any = res
		if err != nil {
			we, ok := errors.AsType[*weberrors.WebError](err)
			if !ok {
				// It's not an expected error
				slog.Error("handler raised error", "err", err)
				we = weberrors.ErrServerInternal
			}
			final = errorResponse{we.Note}
			w.WriteHeader(we.Status)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(final); err != nil {
			// We shouldn't be failing JSON serializations
			// That is something that can be accounted for
			// at compile-time for the most part
			panic(err)
		}

	})
}
