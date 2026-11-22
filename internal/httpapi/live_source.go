package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type liveFormat struct {
	Name   string `json:"format_name"`
	Codecs []struct {
		Name  string `json:"codec_name"`
		Base  string `json:"base_url"`
		Hosts []struct {
			Host  string `json:"host"`
			Extra string `json:"extra"`
		} `json:"url_info"`
	} `json:"codec"`
}

type liveResult struct {
	Live    bool              `json:"live"`
	Sources map[string]string `json:"sources"`
}
type liveEntry struct {
	result  liveResult
	expires time.Time
}
type liveCall struct {
	done   chan struct{}
	result liveResult
	err    error
}
type liveResolver struct {
	endpoint string
	client   *http.Client
	mu       sync.Mutex
	cache    map[uint64]liveEntry
	pending  map[uint64]*liveCall
}

func newLiveResolver(base string, client *http.Client) *liveResolver {
	if client == nil {
		client = &http.Client{Timeout: 6 * time.Second, Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, MaxIdleConns: 32, MaxIdleConnsPerHost: 8, IdleConnTimeout: 60 * time.Second, TLSHandshakeTimeout: 3 * time.Second}}
	}
	return &liveResolver{endpoint: base, client: client, cache: make(map[uint64]liveEntry), pending: make(map[uint64]*liveCall)}
}
func (s *liveResolver) serve(w http.ResponseWriter, r *http.Request) {
	room, err := strconv.ParseUint(r.URL.Query().Get("room_id"), 10, 63)
	if err != nil || room == 0 {
		http.Error(w, "直播间编号无效", http.StatusBadRequest)
		return
	}
	result, err := s.resolve(r.Context(), room)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, result)
}
func (s *liveResolver) resolve(ctx context.Context, room uint64) (liveResult, error) {
	s.mu.Lock()
	if entry, ok := s.cache[room]; ok && time.Now().Before(entry.expires) {
		s.mu.Unlock()
		return entry.result, nil
	}
	call := s.pending[room]
	if call == nil {
		call = &liveCall{done: make(chan struct{})}
		s.pending[room] = call
		go s.load(context.WithoutCancel(ctx), room, call)
	}
	s.mu.Unlock()
	select {
	case <-ctx.Done():
		return liveResult{}, ctx.Err()
	case <-call.done:
		return call.result, call.err
	}
}
func (s *liveResolver) load(ctx context.Context, room uint64, call *liveCall) {
	call.result, call.err = s.fetch(ctx, room)
	s.mu.Lock()
	defer s.mu.Unlock()
	if call.err == nil {
		ttl := 5 * time.Second
		if call.result.Live {
			ttl = 15 * time.Second
		}
		if len(s.cache) >= 512 {
			for key := range s.cache {
				delete(s.cache, key)
				break
			}
		}
		s.cache[room] = liveEntry{call.result, time.Now().Add(ttl)}
	}
	delete(s.pending, room)
	close(call.done)
}
func (s *liveResolver) fetch(parent context.Context, room uint64) (liveResult, error) {
	ctx, cancel := context.WithTimeout(parent, 6*time.Second)
	defer cancel()
	endpoint := s.endpoint + "/xlive/web-room/v2/index/getRoomPlayInfo?protocol=0,1&format=0,1,2&codec=0&qn=80&platform=web&room_id=" + strconv.FormatUint(room, 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return liveResult{}, errors.New("直播源请求失败")
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return liveResult{}, errors.New("直播源连接失败")
	}
	defer resp.Body.Close()
	var result struct {
		Code *int `json:"code"`
		Data struct {
			Status int `json:"live_status"`
			Info   struct {
				Play struct {
					Streams []struct {
						Formats []liveFormat `json:"format"`
					} `json:"stream"`
				} `json:"playurl"`
			} `json:"playurl_info"`
		} `json:"data"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result) != nil || result.Code == nil || *result.Code != 0 {
		return liveResult{}, errors.New("直播平台暂未提供播放地址")
	}
	sources := map[string]string{}
	for _, stream := range result.Data.Info.Play.Streams {
		for _, f := range stream.Formats {
			kind := "hls"
			switch f.Name {
			case "flv":
				kind = "flv"
			case "ts", "fmp4":
			default:
				continue
			}
			for _, codec := range f.Codecs {
				for _, host := range codec.Hosts {
					u, err := url.Parse(host.Host + codec.Base + host.Extra)
					if err == nil && u.Scheme == "https" && u.Hostname() != "" && sources[kind] == "" {
						sources[kind] = u.String()
					}
				}
			}
		}
	}
	return liveResult{Live: result.Data.Status == 1, Sources: sources}, nil
}
