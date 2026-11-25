package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scthy/maku/internal/pubsub"
	"github.com/scthy/maku/internal/store"
)

type rank struct {
	Slug    string
	Status  string
	Created time.Time
	pubsub.Activity
}
type roomIndex struct {
	Ranks    []rank
	Degraded bool
}
type cached[T any] struct {
	value T
	until time.Time
}
type flight[T any] struct {
	done  chan struct{}
	value T
	err   error
}
type flights[T any] struct {
	mu      sync.Mutex
	pending map[string]*flight[T]
}

func (g *flights[T]) do(ctx context.Context, key string, load func() (T, error)) (T, error) {
	g.mu.Lock()
	if f := g.pending[key]; f != nil {
		g.mu.Unlock()
		select {
		case <-f.done:
			return f.value, f.err
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		}
	}
	if g.pending == nil {
		g.pending = make(map[string]*flight[T])
	}
	f := &flight[T]{done: make(chan struct{})}
	g.pending[key] = f
	g.mu.Unlock()
	f.value, f.err = load()
	g.mu.Lock()
	delete(g.pending, key)
	close(f.done)
	g.mu.Unlock()
	return f.value, f.err
}

type plazaCache struct {
	mu         sync.Mutex
	generation uint64
	revision   string
	index      map[string]cached[roomIndex]
	details    map[string]cached[store.Room]
	indexes    flights[roomIndex]
	batches    flights[map[string]store.Room]
}

func (c *plazaCache) invalidate(slug string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	clear(c.index)
	delete(c.details, slug)
}
func (h *ChatHandler) invalidateRoom(ctx context.Context, slug string) {
	h.plaza.invalidate(slug)
	if err := h.sessions.BumpRoomRevision(ctx, slug); err != nil {
		slog.Warn("Invalidate room metadata on peer gateways", "error", err)
	}
}

func (c *plazaCache) observe(revision string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if revision != c.revision {
		previous, _, _ := strings.Cut(c.revision, ":")
		_, change, _ := strings.Cut(revision, ":")
		parent, slug, ok := strings.Cut(change, ":")
		c.revision = revision
		c.generation++
		clear(c.index)
		if ok && parent == previous {
			delete(c.details, slug)
		} else {
			clear(c.details)
		}
	}
}
func (h *ChatHandler) rankings(ctx context.Context, db *pgxpool.Pool, order, status string) (roomIndex, error) {
	c := &h.plaza
	key := order + "/" + status
	if db == nil {
		key += "/offline"
	}
	c.mu.Lock()
	generation := c.generation
	c.mu.Unlock()
	return c.indexes.do(ctx, fmt.Sprintf("%d/%s", generation, key), func() (roomIndex, error) {
		c.mu.Lock()
		entry := c.index[key]
		c.mu.Unlock()
		if time.Now().Before(entry.until) {
			return entry.value, nil
		}
		activity, err := h.sessions.Activity(ctx)
		if err != nil {
			return roomIndex{}, err
		}
		ranks := make(map[string]rank, len(activity))
		for slug, a := range activity {
			ranks[slug] = rank{Slug: slug, Status: "live", Activity: a}
		}
		degraded := db == nil
		if db != nil {
			// Only active slugs and the first page of inactive candidates can reach the top 50.
			active := make([]string, 0, len(activity))
			for slug := range activity {
				active = append(active, slug)
			}
			baselineOrder := "slug COLLATE \"C\""
			if order == "latest" {
				baselineOrder = "created_at DESC, " + baselineOrder
			}
			query := `SELECT slug,status,created_at FROM rooms WHERE slug=ANY($1)
 UNION (SELECT slug,status,created_at FROM rooms WHERE $2='all' OR status='live' ORDER BY ` + baselineOrder + ` LIMIT 50)`
			lookup, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			rows, e := db.Query(lookup, query, active, status)
			if e == nil {
				var rooms []rank
				rooms, e = pgx.CollectRows(rows, func(row pgx.CollectableRow) (rank, error) {
					var v rank
					err := row.Scan(&v.Slug, &v.Status, &v.Created)
					return v, err
				})
				if e == nil {
					for _, v := range rooms {
						v.Activity = activity[v.Slug]
						ranks[v.Slug] = v
					}
				}
			}
			degraded = e != nil
		}
		idx := roomIndex{Degraded: degraded}
		for _, v := range ranks {
			if status == "all" || v.Status == "live" {
				idx.Ranks = append(idx.Ranks, v)
			}
		}
		sort.Slice(idx.Ranks, func(i, j int) bool {
			a, b := idx.Ranks[i], idx.Ranks[j]
			switch order {
			case "hot":
				if a.Heat != b.Heat {
					return a.Heat > b.Heat
				}
			case "online":
				if a.Online != b.Online {
					return a.Online > b.Online
				}
			case "latest":
				if !a.Created.Equal(b.Created) {
					return a.Created.After(b.Created)
				}
			}
			return a.Slug < b.Slug
		})
		idx.Ranks = idx.Ranks[:min(len(idx.Ranks), 50)]
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.generation == generation {
			if c.index == nil {
				c.index = make(map[string]cached[roomIndex])
			}
			c.index[key] = cached[roomIndex]{idx, time.Now().Add(2 * time.Second)}
		}
		return idx, nil
	})
}
func roomDetails(ctx context.Context, db *pgxpool.Pool, keys []string) (map[string]store.Room, error) {
	rows, err := db.Query(ctx, `SELECT slug,owner_id,title,description,active_session,bilibili_room_id,notice,status,slow_mode_seconds,mute_all,created_at,updated_at FROM rooms WHERE slug=ANY($1)`, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]store.Room)
	for rows.Next() {
		var v store.Room
		if err := rows.Scan(&v.Slug, &v.OwnerID, &v.Title, &v.Description, &v.ActiveSession, &v.Bilibili, &v.Notice, &v.Status, &v.Slow, &v.MuteAll, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out[v.Slug] = v
	}
	return out, rows.Err()
}
func (c *plazaCache) getDetails(ctx context.Context, ranks []rank, load func([]string) (map[string]store.Room, error)) (map[string]store.Room, error) {
	out := make(map[string]store.Room)
	var missing []string
	c.mu.Lock()
	generation := c.generation
	for _, v := range ranks {
		if e := c.details[v.Slug]; time.Now().Before(e.until) {
			out[v.Slug] = e.value
		} else {
			missing = append(missing, v.Slug)
		}
	}
	c.mu.Unlock()
	if len(missing) == 0 {
		return out, nil
	}
	sort.Strings(missing)
	batch, err := c.batches.do(ctx, fmt.Sprintf("%d/%s", generation, strings.Join(missing, ",")), func() (map[string]store.Room, error) {
		c.mu.Lock()
		found := make(map[string]store.Room)
		var keys []string
		for _, slug := range missing {
			if e := c.details[slug]; time.Now().Before(e.until) {
				found[slug] = e.value
			} else {
				keys = append(keys, slug)
			}
		}
		c.mu.Unlock()
		if len(keys) == 0 {
			return found, nil
		}
		rooms, e := load(keys)
		if e != nil {
			return nil, e
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.details == nil {
			c.details = make(map[string]cached[store.Room])
		}
		for _, slug := range keys {
			v := rooms[slug]
			if v.Slug == "" {
				v.Slug = slug
				v.Title = slug
				v.Status = "live"
			}
			found[slug] = v
			if c.generation == generation {
				if len(c.details) >= 512 {
					for old := range c.details {
						delete(c.details, old)
						break
					}
				}
				c.details[slug] = cached[store.Room]{v, time.Now().Add(20 * time.Second)}
			}
		}
		return found, nil
	})
	for slug, v := range batch {
		out[slug] = v
	}
	return out, err
}
func (h *ChatHandler) square(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	order, status := q.Get("sort"), q.Get("status")
	if order == "" {
		order = "hot"
	}
	if status == "" {
		status = "all"
	}
	limit, ok := listLimit(r, 20, 50)
	if !ok || (order != "hot" && order != "online" && order != "latest") || (status != "live" && status != "all") {
		w.WriteHeader(400)
		return
	}
	ctx := r.Context()
	db := h.db()
	revision, err := h.sessions.RoomRevision(ctx)
	if err != nil {
		dbError(w, err)
		return
	}
	h.plaza.observe(revision)
	idx, err := h.rankings(ctx, db, order, status)
	if err != nil {
		dbError(w, err)
		return
	}
	ranks := idx.Ranks[:min(limit, len(idx.Ranks))]
	var details map[string]store.Room
	if !idx.Degraded {
		details, err = h.plaza.getDetails(ctx, ranks, func(keys []string) (map[string]store.Room, error) {
			lookup, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			return roomDetails(lookup, db, keys)
		})
		if err != nil {
			idx, err = h.rankings(ctx, nil, order, status)
			if err != nil {
				dbError(w, err)
				return
			}
			ranks = idx.Ranks[:min(limit, len(idx.Ranks))]
			details = nil
		}
	}
	type summary struct {
		store.Room
		pubsub.Activity
		Degraded bool `json:"degraded,omitempty"`
	}
	rooms := make([]summary, 0, len(ranks))
	for _, v := range ranks {
		room := details[v.Slug]
		if room.Slug == "" {
			if idx.Degraded && v.Activity == (pubsub.Activity{}) {
				continue
			}
			room.Slug = v.Slug
			room.Title = v.Slug
			room.Status = v.Status
		}
		if room.ActiveSession == "" {
			room.ActiveSession = v.BusiestSession
			if room.ActiveSession == "" {
				room.ActiveSession = "live"
			}
		}
		rooms = append(rooms, summary{room, v.Activity, idx.Degraded})
	}
	writeJSON(w, map[string]any{"rooms": rooms, "degraded": idx.Degraded})
}
