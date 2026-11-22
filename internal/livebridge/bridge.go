package livebridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/scthy/maku/internal/protocol"
)

const (
	origin         = "https://live.bilibili.com"
	agent          = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/124.0 Safari/537.36"
	requestTimeout = 10 * time.Second
)

var errAuth = errors.New("Bilibili credentials rejected")

type Config struct {
	APIBase, LiveBase string
	HTTPClient        *http.Client
	Dialer            *websocket.Dialer
	Logger            *slog.Logger
}
type upstream struct {
	cancel    context.CancelFunc
	listeners map[string]func(protocol.LiveDanmaku)
}
type credentials struct {
	buvid, key string
	expires    time.Time
}
type Manager struct {
	cfg     Config
	life    context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	rooms   map[string]uint64
	streams map[uint64]*upstream
	wg      sync.WaitGroup
	cached  *credentials
	loading chan struct{}
}

func New(cfg Config) *Manager {
	if cfg.APIBase == "" {
		cfg.APIBase = "https://api.bilibili.com"
	}
	if cfg.LiveBase == "" {
		cfg.LiveBase = "https://api.live.bilibili.com"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: requestTimeout}
	}
	if cfg.Dialer == nil {
		cfg.Dialer = websocket.DefaultDialer
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{cfg: cfg, life: ctx, cancel: cancel, rooms: make(map[string]uint64), streams: make(map[uint64]*upstream)}
}
func (m *Manager) Subscribe(room string, id uint64, deliver func(protocol.LiveDanmaku)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.life.Err() != nil || id == 0 || deliver == nil {
		return
	}
	if old, ok := m.rooms[room]; ok && old != id {
		m.remove(room, old)
	}
	s := m.streams[id]
	if s == nil {
		ctx, cancel := context.WithCancel(m.life)
		s = &upstream{cancel: cancel, listeners: make(map[string]func(protocol.LiveDanmaku))}
		m.streams[id] = s
		m.wg.Add(1)
		go func() { defer m.wg.Done(); m.run(ctx, id, s) }()
	}
	s.listeners[room] = deliver
	m.rooms[room] = id
}
func (m *Manager) remove(room string, id uint64) {
	s := m.streams[id]
	delete(s.listeners, room)
	delete(m.rooms, room)
	if len(s.listeners) == 0 {
		delete(m.streams, id)
		s.cancel()
	}
}
func (m *Manager) Unsubscribe(room string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, ok := m.rooms[room]; ok {
		m.remove(room, id)
	}
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.cancel()
	clear(m.rooms)
	clear(m.streams)
	m.mu.Unlock()
	m.wg.Wait()
}
func (m *Manager) run(ctx context.Context, id uint64, s *upstream) {
	delay := time.Second
	deliver := func(msg protocol.LiveDanmaku) {
		m.mu.Lock()
		listeners := make([]func(protocol.LiveDanmaku), 0, len(s.listeners))
		for _, f := range s.listeners {
			listeners = append(listeners, f)
		}
		m.mu.Unlock()
		for _, f := range listeners {
			if ctx.Err() != nil {
				return
			}
			f(msg)
		}
	}
	for ctx.Err() == nil {
		start := time.Now()
		err := m.connect(ctx, id, deliver)
		if ctx.Err() != nil {
			return
		}
		m.cfg.Logger.Warn("Bilibili connection interrupted", "room", id, "error", err)
		if time.Since(start) > time.Minute {
			delay = time.Second
		}
		timer := time.NewTimer(delay/2 + rand.N(delay/2))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		delay = min(2*delay, 30*time.Second)
	}
}

type apiError int

func (e apiError) Error() string { return fmt.Sprintf("Bilibili API code %d", int(e)) }
func (m *Manager) get(ctx context.Context, endpoint, buvid string, out any) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", agent)
	req.Header.Set("Referer", origin+"/")
	if buvid != "" {
		req.AddCookie(&http.Cookie{Name: "buvid3", Value: buvid})
	}
	res, err := m.cfg.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("Bilibili HTTP status %d", res.StatusCode)
	}
	var envelope struct {
		Code int
		Data json.RawMessage
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, maxPayload)).Decode(&envelope); err != nil {
		return err
	}
	// Anonymous nav requests may return -101 together with valid WBI images.
	if envelope.Code != 0 && !(envelope.Code == -101 && endpoint == m.cfg.APIBase+"/x/web-interface/nav") {
		return apiError(envelope.Code)
	}
	return json.Unmarshal(envelope.Data, out)
}
func (m *Manager) load(ctx context.Context) (*credentials, error) {
	var finger struct {
		B3 string `json:"b_3"`
	}
	if err := m.get(ctx, m.cfg.APIBase+"/x/frontend/finger/spi", "", &finger); err != nil {
		return nil, err
	}
	if finger.B3 == "" {
		return nil, errors.New("empty buvid3")
	}
	var nav struct {
		Images struct {
			Img string `json:"img_url"`
			Sub string `json:"sub_url"`
		} `json:"wbi_img"`
	}
	if err := m.get(ctx, m.cfg.APIBase+"/x/web-interface/nav", finger.B3, &nav); err != nil {
		return nil, err
	}
	key, err := mixin(nav.Images.Img, nav.Images.Sub)
	if err != nil {
		return nil, err
	}
	return &credentials{finger.B3, key, time.Now().Add(10 * time.Minute)}, nil
}
func (m *Manager) credentials(ctx context.Context) (*credentials, error) {
	for ctx.Err() == nil {
		m.mu.Lock()
		if c := m.cached; c != nil && time.Now().Before(c.expires) {
			m.mu.Unlock()
			return c, nil
		}
		if wait := m.loading; wait != nil {
			m.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-wait:
				continue
			}
		}
		m.loading = make(chan struct{})
		m.mu.Unlock()
		c, err := m.load(ctx)
		m.mu.Lock()
		m.cached = c
		close(m.loading)
		m.loading = nil
		m.mu.Unlock()
		return c, err
	}
	return nil, ctx.Err()
}
func (m *Manager) connect(ctx context.Context, id uint64, deliver func(protocol.LiveDanmaku)) (err error) {
	var room struct {
		ID uint64 `json:"room_id"`
	}
	c, err := m.credentials(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if errors.Is(err, errAuth) || errors.Is(err, apiError(-352)) {
			m.mu.Lock()
			if m.cached == c {
				m.cached = nil
			}
			m.mu.Unlock()
		}
	}()
	endpoint := fmt.Sprintf("%s/xlive/web-room/v2/index/getRoomPlayInfo?protocol=0,1&format=0,1,2&codec=0&qn=80&platform=web&room_id=%d", m.cfg.LiveBase, id)
	if err = m.get(ctx, endpoint, c.buvid, &room); err != nil {
		return err
	}
	if room.ID == 0 {
		return errors.New("missing real room ID")
	}
	query := url.Values{"id": {strconv.FormatUint(room.ID, 10)}, "type": {"0"}, "web_location": {"444.8"}, "wts": {strconv.FormatInt(time.Now().Unix(), 10)}}
	var info struct {
		Token string
		Hosts []struct {
			Host string
			Port int `json:"wss_port"`
		} `json:"host_list"`
	}
	if err = m.get(ctx, m.cfg.LiveBase+"/xlive/web-room/v1/index/getDanmuInfo?"+signed(query, c.key), c.buvid, &info); err != nil {
		return err
	}
	if info.Token == "" || len(info.Hosts) == 0 {
		return errors.New("missing live host or token")
	}
	auth, err := json.Marshal(map[string]any{"uid": 0, "roomid": room.ID, "protover": 2, "buvid": c.buvid, "platform": "web", "type": 2, "key": info.Token})
	if err != nil {
		return err
	}
	for _, h := range info.Hosts {
		endpoint := url.URL{Scheme: "wss", Host: net.JoinHostPort(h.Host, strconv.Itoa(h.Port)), Path: "/sub"}
		dialCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		conn, _, dialErr := m.cfg.Dialer.DialContext(dialCtx, endpoint.String(), http.Header{"Origin": {origin}, "User-Agent": {agent}})
		cancel()
		if dialErr == nil {
			return session(ctx, conn, auth, deliver)
		}
		err = dialErr
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return err
}
func write(conn *websocket.Conn, op uint32, body []byte) error {
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, packet(1, op, body))
}
func session(parent context.Context, conn *websocket.Conn, auth []byte, deliver func(protocol.LiveDanmaku)) error {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	authenticated := make(chan struct{})
	go func() {
		defer close(done)
		defer conn.Close()
		select {
		case <-ctx.Done():
			return
		case <-authenticated:
		}
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if write(conn, 2, nil) != nil {
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-done }()
	conn.SetReadLimit(maxPayload)
	if err := write(conn, 7, auth); err != nil {
		return err
	}
	ready := false
	if err := conn.SetReadDeadline(time.Now().Add(requestTimeout)); err != nil {
		return err
	}
	for ctx.Err() == nil {
		if ready {
			if err := conn.SetReadDeadline(time.Now().Add(75 * time.Second)); err != nil {
				return err
			}
		}
		typ, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		if typ != websocket.BinaryMessage {
			continue
		}
		err = decode(data, func(op uint32, body []byte) error {
			switch op {
			case 8:
				var ack struct{ Code *int }
				if json.Unmarshal(body, &ack) != nil || ack.Code == nil || *ack.Code != 0 {
					return errAuth
				}
				if !ready {
					ready = true
					close(authenticated)
				}
			case 5:
				if ready {
					if msg, ok := parseMessage(body); ok {
						deliver(msg)
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return ctx.Err()
}
