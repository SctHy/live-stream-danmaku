package commerce

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/scthy/maku/internal/store"
)

type customEffect struct{ standard }

func (customEffect) Type() string { return "confetti" }

type customProvider struct{ *Sandbox }

func (customProvider) Name() string { return "custom" }
func TestGiftPlugins(t *testing.T) {
	catalog := DefaultCatalog()
	if len(catalog.List()) != 5 {
		t.Fatal(catalog)
	}
	effects := DefaultEffects()
	if err := effects.Register(customEffect{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "catalog.json")
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`[{"code":"sample","name":"Sample","price":1,"effect_type":"confetti"}]`, true},
		{`[{"code":"sample","name":"Sample","price":0,"effect_type":"standard"}]`, false},
		{`[{"code":"sample","name":"Sample","price":1,"effect_type":"missing"}]`, false},
	} {
		if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
			t.Fatal(err)
		}
		file, err := LoadCatalog(path, effects)
		if (err == nil) != tc.valid {
			t.Fatal(err)
		}
		if tc.valid {
			spec, ok := file.Get("sample")
			if !ok {
				t.Fatal("missing sample")
			}
			if _, err := effects[spec.EffectType].Prepare(spec, GiftRequest{Quantity: 1}); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, tc := range []struct {
		code  string
		q     int
		text  string
		valid bool
	}{
		{"glowstick", 2, "", true}, {"coffee", 100, "", false}, {"sc", 2, "hi", false}, {"sc", 1, " ", false}, {"sc", 1, "你好", true},
	} {
		s, _ := catalog.Get(tc.code)
		_, err := effects[s.EffectType].Prepare(s, GiftRequest{Quantity: tc.q, Message: tc.text})
		if (err == nil) != tc.valid {
			t.Fatal(tc, err)
		}
	}
}
func TestPaymentPlugins(t *testing.T) {
	s := NewSandbox("test-secret")
	p := ProviderRegistry{}
	custom := customProvider{s}
	if err := p.Register(custom); err != nil {
		t.Fatal(err)
	}
	if err := p.Register(custom); err == nil {
		t.Fatal("accepted duplicate")
	}
	if checkout, err := p["custom"].Create(context.Background(), store.Recharge{OrderNo: "order"}); err != nil || checkout.URL == "" {
		t.Fatal(checkout, err)
	}
	body := []byte(`{"action":"pay","order_no":"o","provider_trade_no":"t"}`)
	if _, err := s.VerifyWebhook(s.Sign(body), body); err != nil {
		t.Fatal(err)
	}
	if _, err := s.VerifyWebhook(s.Sign([]byte("altered")), body); err == nil {
		t.Fatal("accepted mismatched signature")
	}
}
