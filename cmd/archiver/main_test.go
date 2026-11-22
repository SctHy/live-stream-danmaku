package main

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scthy/maku/internal/metrics"
)

func TestAdminHandler(t *testing.T) {
	var dbErr error
	h := adminHandler(metrics.NewArchiveMetrics().Handler(), func(ctx context.Context) error {
		d, ok := ctx.Deadline()
		if !ok || time.Until(d) > time.Second {
			t.Error("missing probe deadline")
		}
		return dbErr
	})
	for _, tt := range []struct {
		path string
		code int
	}{{"/metrics", 200}, {"/livez", 200}, {"/readyz", 200}, {"/readyz", 503}, {"/livez", 200}} {
		if tt.code == 503 {
			dbErr = errors.New("offline")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", tt.path, nil))
		if w.Code != tt.code || tt.path == "/metrics" && !strings.Contains(w.Body.String(), "danmaku_archive_written_total") {
			t.Error(tt, w.Code)
		}
	}
}
