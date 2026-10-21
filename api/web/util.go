package web

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/netip"

	"github.com/go-playground/validator/v10"
	"github.com/minishd/minnatropolis/datastore"
)

var validate *validator.Validate = validator.New(
	validator.WithRequiredStructEnabled(),
	validator.WithTagNameFuncBlankOmit(),
)

const kSession = "session"

func SetAuthCookie(w http.ResponseWriter, token string) {
	var maxAge int
	if token != "" {
		maxAge = 3600 * 24 * 7
	} else {
		// we are clearing the cookie,
		// so set max age to 0 as well
		maxAge = 0
	}

	cookie := &http.Cookie{
		Name:        kSession,
		Value:       token,
		MaxAge:      maxAge,
		SameSite:    http.SameSiteStrictMode,
		HttpOnly:    true,
		Secure:      true,
		Partitioned: true,
		Path:        "/",
	}
	http.SetCookie(w, cookie)
}

// Reads the IP of a request
func GetIP(r *http.Request) netip.Addr {
	// First, try x-forwarded-for header
	xff := r.Header.Get("x-forwarded-for")
	addr, err := netip.ParseAddr(xff)
	if err == nil {
		return addr
	}

	// If that doesn't work, use connection IP
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		log.Println("couldn't read IP from request", err)
		return netip.IPv4Unspecified()
	}

	// That worked, return the connection IP
	return ap.Addr()
}

// Reads and looks up a provided session token
func GetAuth(ds *datastore.DataStore, r *http.Request) (st *datastore.SessionToken, err error) {
	// Read token from cookie
	cookie, err_ := r.Cookie(kSession)
	if err_ != nil {
		// [http.Request.Cookie] only returns an error
		// if the cookie was not found.
		// (We do not consider that an error)
		return
	}
	token := cookie.Value

	// Skip if it's obviously not valid
	if token == "" {
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

// Parses the body of a request from JSON
func ParseReq[Req any](r *http.Request) (req Req, err error) {
	// Only proceed if Content-Type is JSON
	// TODO: make this more lenient?
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		err = ErrNotJSON
		return
	}

	return ParseJSON[Req](r.Body)
}

// Parses and validates JSON
func ParseJSON[Req any](data io.Reader) (req Req, err error) {
	// Decode body
	dec := json.NewDecoder(data)
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
