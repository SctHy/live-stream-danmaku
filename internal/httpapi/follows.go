package httpapi

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/scthy/maku/internal/pubsub"
)

func (h *ChatHandler) follow(w http.ResponseWriter, r *http.Request) {
	login, ok := h.requireLogin(w, r)
	if !ok {
		return
	}
	db := h.db()
	if db == nil {
		w.WriteHeader(503)
		return
	}
	query := "DELETE FROM room_follows WHERE user_id=$1 AND room_slug=$2"
	if r.Method == "POST" {
		query = "INSERT INTO room_follows(user_id,room_slug) VALUES($1,$2) ON CONFLICT DO NOTHING"
	}
	if _, err := db.Exec(r.Context(), query, login.UserID, r.PathValue("room")); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == "room_follows_room_slug_fkey" {
			w.WriteHeader(404)
			return
		}
		dbError(w, err)
		return
	}
	w.WriteHeader(204)
}
func (h *ChatHandler) following(w http.ResponseWriter, r *http.Request) {
	login, ok := h.requireLogin(w, r)
	if !ok {
		return
	}
	db := h.db()
	if db == nil {
		w.WriteHeader(503)
		return
	}
	rows, err := db.Query(r.Context(), `SELECT r.slug,r.title,r.status,r.active_session,r.cover_theme FROM rooms r JOIN room_follows f ON f.room_slug=r.slug WHERE f.user_id=$1 ORDER BY (r.status='live') DESC,f.created_at DESC,r.slug`, login.UserID)
	if err != nil {
		dbError(w, err)
		return
	}
	type followed struct {
		Slug    string `json:"slug"`
		Title   string `json:"title"`
		Status  string `json:"status"`
		Session string `json:"active_session"`
		Theme   string `json:"cover_theme"`
		pubsub.Activity
	}
	rooms, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (followed, error) {
		var v followed
		err := row.Scan(&v.Slug, &v.Title, &v.Status, &v.Session, &v.Theme)
		return v, err
	})
	if err != nil {
		dbError(w, err)
		return
	}
	activity, err := h.sessions.Activity(r.Context())
	if err != nil {
		dbError(w, err)
		return
	}
	for i := range rooms {
		rooms[i].Activity = activity[rooms[i].Slug]
	}
	writeJSON(w, map[string]any{"rooms": rooms})
}
