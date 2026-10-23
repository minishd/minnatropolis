package admin

import (
	"net/http"

	"github.com/minishd/minnatropolis/api/chat"
	"github.com/minishd/minnatropolis/api/room"
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

func (h *Handlers) HandleHiAdmin(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error) {
	w.Write([]byte("hi admin"))
	return
}

func (h *Handlers) HandleHiModerator(w http.ResponseWriter, r *http.Request, session *datastore.SessionToken) (err error) {
	w.Write([]byte("hi moderator"))
	return
}
