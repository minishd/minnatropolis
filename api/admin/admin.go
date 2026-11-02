package admin

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/minishd/minnatropolis/api/chat"
	"github.com/minishd/minnatropolis/api/room"
	"github.com/minishd/minnatropolis/api/web"
	"github.com/minishd/minnatropolis/datastore"
)

type Handlers struct {
	ds *datastore.DataStore
	rh *room.Handler
	ch *chat.Handler
}

func NewHandlers(ds *datastore.DataStore, rh *room.Handler, ch *chat.Handler) *Handlers {
	return &Handlers{ds, rh, ch}
}

func (h *Handlers) HandleSearchUsers(w http.ResponseWriter, r *http.Request, st *datastore.SessionToken) (err error) {
	type searchUsersReq struct {
		Filters struct {
			Username, Email string             // optional "contains" filter
			UserID          uuid.UUID          // optional "equals" filter
			HasPunishment   web.PunishmentType // optional "equals" filter
			SharesIPWith    uuid.UUID          // optional "equals" filter
		}

		Sort struct {
			By    usersSort `schema:"required"`
			Order sortOrder `schema:"required"`
		}

		Page struct {
			PerPage uint `schema:"required" validate:"max=100"`
			Offset  uint `schema:"required"`
		}
	}

	// Parse request
	req, err := web.ParseQuery[searchUsersReq](r)
	if err != nil {
		return
	}

	// Return it
	web.SendResOK(w, req)

	return
}

func (h *Handlers) HandleHiAdmin(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error) {
	w.Write([]byte("hi admin"))
	return
}

func (h *Handlers) HandleHiModerator(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error) {
	w.Write([]byte("hi moderator"))
	return
}
