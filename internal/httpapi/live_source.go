package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
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

// liveSource resolves numeric Bilibili room IDs for the local screening-room demo.
func liveSource(w http.ResponseWriter, r *http.Request) {
	room, err := strconv.ParseUint(r.URL.Query().Get("room_id"), 10, 63)
	if err != nil || room == 0 {
		http.Error(w, "直播间编号无效", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	endpoint := "https://api.live.bilibili.com/xlive/web-room/v2/index/getRoomPlayInfo?protocol=0,1&format=0,1,2&codec=0&qn=80&platform=web&room_id=" + strconv.FormatUint(room, 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		http.Error(w, "直播源请求失败", 502)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "直播源连接失败", 502)
		return
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
		http.Error(w, "直播平台暂未提供播放地址", 502)
		return
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
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{"live": result.Data.Status == 1, "sources": sources})
}
