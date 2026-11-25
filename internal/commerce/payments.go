package commerce

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/scthy/maku/internal/store"
)

type Package struct {
	Code   string `json:"package_code"`
	Amount int64  `json:"amount_cents"`
	Coins  int64  `json:"coins"`
}

func Packages() []Package {
	return []Package{{"coins_600", 600, 600}, {"coins_3000", 3000, 3000}, {"coins_6800", 6800, 6800}}
}

type CheckoutInfo struct {
	URL string `json:"url"`
}
type WebhookEvent struct {
	OrderNo string `json:"order_no"`
	Trade   string `json:"provider_trade_no"`
	Action  string `json:"action"`
}
type PaymentProvider interface {
	Name() string
	Create(context.Context, store.Recharge) (CheckoutInfo, error)
	VerifyWebhook(http.Header, []byte) (WebhookEvent, error)
}
type ProviderRegistry map[string]PaymentProvider

func (r ProviderRegistry) Register(p PaymentProvider) error {
	if p == nil || p.Name() == "" || r[p.Name()] != nil {
		return errors.New("invalid or duplicate payment provider")
	}
	r[p.Name()] = p
	return nil
}
func (r ProviderRegistry) Names() []string { return slices.Sorted(maps.Keys(r)) }

type Sandbox struct{ secret string }

func NewSandbox(secret string) *Sandbox {
	if secret == "" {
		secret = rand.Text()
	}
	return &Sandbox{secret}
}
func (*Sandbox) Name() string { return "sandbox" }
func (*Sandbox) Create(_ context.Context, r store.Recharge) (CheckoutInfo, error) {
	return CheckoutInfo{"/api/wallet/recharges/" + r.OrderNo + "/simulate"}, nil
}
func (s *Sandbox) mac(b []byte) []byte {
	h := hmac.New(sha256.New, []byte(s.secret))
	h.Write(b)
	return h.Sum(nil)
}
func (s *Sandbox) Sign(b []byte) http.Header {
	h := http.Header{}
	h.Set("X-Payment-Signature", hex.EncodeToString(s.mac(b)))
	return h
}
func (s *Sandbox) VerifyWebhook(h http.Header, b []byte) (WebhookEvent, error) {
	sig, err := hex.DecodeString(h.Get("X-Payment-Signature"))
	if err != nil || subtle.ConstantTimeCompare(sig, s.mac(b)) != 1 {
		return WebhookEvent{}, errors.New("invalid signature")
	}
	var e WebhookEvent
	if json.Unmarshal(b, &e) != nil || e.OrderNo == "" || (e.Action != "pay" && e.Action != "close") || (e.Action == "pay" && e.Trade == "") {
		return e, errors.New("invalid webhook event")
	}
	return e, nil
}

type Voucher struct{ codes map[string]int64 }

// Codes use comma-separated CODE or CODE=coins entries.
func NewVoucher(config string) (*Voucher, error) {
	if config == "" {
		config = "MAKU2026=600"
	}
	v := &Voucher{map[string]int64{}}
	for _, part := range strings.Split(config, ",") {
		code, value, has := strings.Cut(strings.TrimSpace(part), "=")
		amount := int64(600)
		var err error
		if has {
			amount, err = strconv.ParseInt(value, 10, 64)
		}
		if err != nil || amount <= 0 || !codePattern.MatchString(code) || v.codes[code] != 0 {
			return nil, errors.New("invalid VOUCHER_CODES")
		}
		v.codes[code] = amount
	}
	return v, nil
}
func (*Voucher) Name() string                       { return "voucher" }
func (v *Voucher) Redeem(code string) (int64, bool) { amount, ok := v.codes[code]; return amount, ok }

var ErrVoucherOnly = errors.New("voucher requires a redemption code")

func (*Voucher) Create(context.Context, store.Recharge) (CheckoutInfo, error) {
	return CheckoutInfo{}, ErrVoucherOnly
}
func (*Voucher) VerifyWebhook(http.Header, []byte) (WebhookEvent, error) {
	return WebhookEvent{}, ErrVoucherOnly
}
