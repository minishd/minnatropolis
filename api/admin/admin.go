package admin

import (
	"net/http"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/minishd/minnatropolis/api/chat"
	"github.com/minishd/minnatropolis/api/room"
	"github.com/minishd/minnatropolis/api/web"
	"github.com/minishd/minnatropolis/api/webtypes"
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
			Username      string                   // optional "starts with" filter
			UserID        uuid.UUID                // optional "equals" filter
			HasPunishment *webtypes.PunishmentType // optional "equals" filter
			SharesIPWith  uuid.UUID                // optional "equals" filter
			HasRole       *webtypes.Role           // optional "equals" filter
		}

		Sort struct {
			By    usersSortBy            `validate:"required"`
			Order webtypes.SortDirection `validate:"required"`
		}

		Page struct {
			PerPage uint64 `validate:"required,max=100"`
			Offset  uint64
		}
	}
	type punishmentDetail struct {
		StartedAt time.Time
		Type      webtypes.PunishmentType
		ExpiresAt *time.Time
		Reason    string
	}
	type userDetail struct {
		ID                uuid.UUID
		CreatedAt         time.Time
		Username          string
		LastVisitAt       time.Time
		LastSeenIP        *netip.Addr
		ModNote           string
		CurrentPunishment *punishmentDetail
		Role              webtypes.Role
	}
	type searchUsersRes struct {
		Results []userDetail
	}

	// Parse request
	req, err := web.ParseQuery[searchUsersReq](r)
	if err != nil {
		return
	}

	// Construct types
	var hp *datastore.PunishmentType
	var hr *datastore.Role
	if req.Filters.HasPunishment != nil {
		wpt := *req.Filters.HasPunishment
		pt := wpt.ToDS()
		hp = &pt
	}
	if req.Filters.HasRole != nil {
		wr := *req.Filters.HasRole
		r := wr.ToDS()
		hr = &r
	}
	filters := datastore.UserSearchFilters{
		Username:      req.Filters.Username,
		UserID:        req.Filters.UserID,
		HasPunishment: hp,
		SharesIPWith:  req.Filters.SharesIPWith,
		HasRole:       hr,
	}
	sort := datastore.UserSearchSort{
		By:        req.Sort.By.ToDS(),
		Direction: req.Sort.Order.ToDS(),
	}

	// Make query
	ctx := r.Context()
	users, err := h.ds.SearchUsers(ctx, filters, sort, req.Page.Offset, req.Page.PerPage)
	if err != nil {
		return
	}

	// Convert
	var results []userDetail
	for _, user := range users {
		var punish *punishmentDetail
		if user.CurrentPunishment != nil {
			p := *user.CurrentPunishment
			wpt := webtypes.WPunishmentType(p.PunishmentType)

			punish = &punishmentDetail{
				StartedAt: p.CreatedAt,
				Type:      wpt,
				ExpiresAt: p.ExpiresAt,
				Reason:    p.Reason,
			}
		}

		wrole := webtypes.WRole(user.Role)
		result := userDetail{
			ID:                user.ID,
			CreatedAt:         user.CreatedAt,
			Username:          user.Username,
			LastVisitAt:       user.LastVisitAt,
			LastSeenIP:        user.LastSeenIP,
			ModNote:           user.ModNote,
			CurrentPunishment: punish,
			Role:              wrole,
		}

		results = append(results, result)
	}

	// Return it
	web.SendResOK(w, results)

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
