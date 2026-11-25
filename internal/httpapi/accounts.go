package httpapi

import (
	"bytes"
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/scthy/maku/internal/pubsub"
)

var slug = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,24}$`)

const loginCookie = "danmaku_login"
const loginTTL = 7 * 24 * time.Hour

func (h *ChatHandler) db() *pgxpool.Pool {
	p, ok := h.history.(interface{ BusinessDB() *pgxpool.Pool })
	if !ok {
		return nil
	}
	return p.BusinessDB()
}
func (h *ChatHandler) businessRoutes(mux *http.ServeMux) {
	routes := map[string]http.HandlerFunc{
		"POST /internal/stream/auth":               h.streamAuth,
		"POST /internal/stream/ready":              h.streamHook,
		"POST /internal/stream/ended":              h.streamHook,
		"GET /api/rooms/{room}/stream-key":         h.streamKey,
		"POST /api/rooms/{room}/stream-key/rotate": h.streamKey,
		"GET /api/rooms/{room}/stream":             h.stream,
		"GET /api/rooms/{room}/replays":            h.replays,
		// The session wildcard lets existing literal history and live routes take precedence.
		"GET /api/rooms/{room}/{session}/{id}":        h.replays,
		"DELETE /api/rooms/{room}/replays/{id}":       h.replays,
		"GET /api/replays/{id}/video":                 h.video,
		"GET /api/rooms/{room}/replays/{id}/danmaku":  h.timeline,
		"POST /api/rooms/{room}/replays/{id}/danmaku": h.timeline,
		"GET /api/rooms/{room}/replays/{id}/density":  h.density,
		"POST /api/rooms/{room}/follow":               h.follow,
		"DELETE /api/rooms/{room}/follow":             h.follow,
		"GET /api/me/following":                       h.following,
		"GET /api/me/preferences":                     h.preferences, "GET /api/plaza/strategies": h.strategies,
		"GET /api/rooms": h.square, "GET /api/gifts/catalog": h.giftCatalog,
		"GET /api/wallet/recharge-packages":              h.packages,
		"POST /api/wallet/recharges":                     h.recharge,
		"GET /api/wallet/recharges/{order_no}":           h.recharge,
		"POST /api/wallet/recharges/{order_no}/simulate": h.recharge,
		"POST /api/payments/webhook/{provider}":          h.webhook,
		"POST /api/wallet/redeem":                        h.redeem,
		"GET /api/wallet":                                h.wallet, "POST /api/wallet/checkin": h.wallet,
		"GET /api/rooms/{room}/{session}/gifts": h.gifts, "POST /api/rooms/{room}/{session}/gifts": h.gifts,
		"GET /api/rooms/{room}/{session}/leaderboard": h.leaderboard, "GET /api/rooms/{room}/{session}/analytics": h.analytics,
		"POST /api/auth/register": h.credentials, "POST /api/auth/login": h.credentials,
		"GET /api/auth/me": h.me, "PATCH /api/auth/me": h.me, "POST /api/auth/logout": h.logout,
		"POST /api/rooms": h.rooms, "GET /api/rooms/{room}": h.rooms, "PATCH /api/rooms/{room}": h.rooms,
		"GET /api/rooms/{room}/moderators": h.moderators, "POST /api/rooms/{room}/moderators": h.moderators,
		"DELETE /api/rooms/{room}/moderators/{user_id}": h.moderators,
		"GET /api/rooms/{room}/{session}/audits":        h.moderationList, "GET /api/rooms/{room}/{session}/mutes": h.moderationList,
	}
	for pattern, fn := range routes {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			if !sameOrigin(r) {
				w.WriteHeader(403)
				return
			}
			for _, k := range []string{"room", "session"} {
				if v := r.PathValue(k); v != "" && !slug.MatchString(v) {
					w.WriteHeader(400)
					return
				}
			}
			// Finish reading the bounded body before opening a transaction or locking a room.
			if r.Method == "POST" || r.Method == "PATCH" {
				body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8192))
				if err != nil {
					w.WriteHeader(400)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
			}
			ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
			defer cancel()
			fn(w, r.WithContext(ctx))
		})
	}
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		w.WriteHeader(400)
		return false
	}
	return true
}
func validText(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	return n >= min && n <= max && !strings.ContainsRune(s, 0) && (min == 0 || strings.TrimSpace(s) != "")
}
func derive(password, salt string) string {
	key, err := pbkdf2.Key(sha256.New, password, []byte(salt), 600000, 32)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(key)
}
func hashPassword(password string) string {
	salt := rand.Text()
	return salt + "$" + derive(password, salt)
}
func checkPassword(password, hash string) bool {
	salt, want, ok := strings.Cut(hash, "$")
	if !ok {
		salt = "missing-account-dummy-salt"
		want = strings.Repeat("0", 64)
	}
	got := derive(password, salt)
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1 && ok
}
func (h *ChatHandler) loginIdentity(r *http.Request) (pubsub.Login, error) {
	c, e := r.Cookie(loginCookie)
	if e != nil || len(c.Value) > 101 {
		return pubsub.Login{}, redis.Nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	return h.sessions.Login(ctx, c.Value)
}
func (h *ChatHandler) requireLogin(w http.ResponseWriter, r *http.Request) (pubsub.Login, bool) {
	v, e := h.loginIdentity(r)
	if e == redis.Nil {
		w.WriteHeader(401)
	} else if e != nil {
		w.WriteHeader(503)
	}
	return v, e == nil
}
func setLoginCookie(w http.ResponseWriter, r *http.Request, token string, age int) {
	http.SetCookie(w, &http.Cookie{Name: loginCookie, Value: token, Path: "/", MaxAge: age, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil})
}
func dbError(w http.ResponseWriter, err error) {
	var p *pgconn.PgError
	switch {
	case errors.As(err, &p) && p.Code == "23505":
		w.WriteHeader(409)
	case errors.Is(err, pgx.ErrNoRows):
		w.WriteHeader(404)
	default:
		w.WriteHeader(503)
	}
}

type accountView struct {
	pubsub.Login
	Authenticated bool   `json:"authenticated"`
	Balance       *int64 `json:"balance"`
}

func (h *ChatHandler) credentials(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password, Nickname string }
	if !decode(w, r, &in) {
		return
	}
	if in.Nickname == "" {
		in.Nickname = in.Username
	}
	if !validText(in.Username, 3, 24) || !validText(in.Password, 6, 72) || !validText(in.Nickname, 1, 24) {
		w.WriteHeader(400)
		return
	}
	db := h.db()
	if db == nil {
		w.WriteHeader(503)
		return
	}
	v := pubsub.Login{Username: in.Username, Nickname: in.Nickname}
	var balance int64
	var err error
	if r.URL.Path == "/api/auth/register" {
		n, e := rand.Int(rand.Reader, big.NewInt((1<<52)-1))
		if e != nil {
			w.WriteHeader(503)
			return
		}
		v.UserID = n.Uint64() + 1
		err = db.QueryRow(r.Context(), "INSERT INTO users(id,username,nickname,password_hash) VALUES($1,$2,$3,$4) RETURNING balance", v.UserID, v.Username, v.Nickname, hashPassword(in.Password)).Scan(&balance)
	} else {
		var hash string
		err = db.QueryRow(r.Context(), "SELECT id::bigint,nickname,password_hash,balance FROM users WHERE username=$1", v.Username).Scan(&v.UserID, &v.Nickname, &hash, &balance)
		if err == nil || errors.Is(err, pgx.ErrNoRows) {
			if !checkPassword(in.Password, hash) {
				w.WriteHeader(401)
				return
			}
		}
	}
	if err != nil {
		dbError(w, err)
		return
	}
	token := strconv.FormatUint(v.UserID, 10) + "." + rand.Text()
	if err = h.sessions.SaveLogin(r.Context(), token, v, loginTTL); err != nil {
		w.WriteHeader(503)
		return
	}
	setLoginCookie(w, r, token, int(loginTTL.Seconds()))
	writeJSON(w, accountView{v, true, &balance})
}
func (h *ChatHandler) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie(loginCookie); e == nil && h.sessions.Logout(r.Context(), c.Value) != nil {
		w.WriteHeader(503)
		return
	}
	setLoginCookie(w, r, "", -1)
	w.WriteHeader(204)
}
func (h *ChatHandler) me(w http.ResponseWriter, r *http.Request) {
	v, e := h.loginIdentity(r)
	if e == redis.Nil && r.Method == "GET" {
		uid, e := h.ensureGuest(w, r)
		if e != nil {
			w.WriteHeader(503)
			return
		}
		writeJSON(w, accountView{pubsub.Login{UserID: uid, IsGuest: true}, false, nil})
		return
	}
	if e == redis.Nil {
		w.WriteHeader(401)
		return
	}
	if e != nil {
		w.WriteHeader(503)
		return
	}
	db := h.db()
	var bal int64
	if r.Method == "PATCH" {
		var in struct{ Nickname string }
		if !decode(w, r, &in) {
			return
		}
		if !validText(in.Nickname, 1, 24) {
			w.WriteHeader(400)
			return
		}
		if db == nil {
			w.WriteHeader(503)
			return
		}
		err := db.QueryRow(r.Context(), "UPDATE users SET nickname=$2 WHERE id=$1 RETURNING balance", v.UserID, in.Nickname).Scan(&bal)
		if err != nil {
			dbError(w, err)
			return
		}
		v.Nickname = in.Nickname
		c, _ := r.Cookie(loginCookie)
		if h.sessions.RefreshLogin(r.Context(), c.Value, v) != nil {
			w.WriteHeader(503)
			return
		}

	} else if db == nil || db.QueryRow(r.Context(), "SELECT balance,nickname FROM users WHERE id=$1", v.UserID).Scan(&bal, &v.Nickname) != nil {
		writeJSON(w, accountView{v, true, nil})
		return
	}
	writeJSON(w, accountView{v, true, &bal})
}
