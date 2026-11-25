package commerce

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

type GiftSpec struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Price      int64  `json:"price"`
	Super      bool   `json:"is_superchat"`
	Pin        int    `json:"pin_seconds"`
	EffectType string `json:"effect_type"`
}
type CatalogProvider interface {
	List() []GiftSpec
	Get(string) (GiftSpec, bool)
}
type StaticCatalog struct{ items []GiftSpec }

func DefaultCatalog() StaticCatalog {
	return StaticCatalog{[]GiftSpec{
		{"glowstick", "荧光棒", 10, false, 0, "combo"}, {"coffee", "咖啡", 50, false, 0, "standard"}, {"rocket", "火箭", 200, false, 0, "standard"}, {"sc", "醒目留言", 100, true, 30, "superchat"}, {"sc_super", "超级醒目留言", 300, true, 60, "superchat"},
	}}
}
func (s StaticCatalog) List() []GiftSpec { return slices.Clone(s.items) }
func (s StaticCatalog) Get(code string) (GiftSpec, bool) {
	for _, v := range s.items {
		if v.Code == code {
			return v, true
		}
	}
	return GiftSpec{}, false
}

type FileCatalog struct{ StaticCatalog }

var codePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func LoadCatalog(path string, effects EffectRegistry) (FileCatalog, error) {
	var c FileCatalog
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(data, &c.items); err != nil {
		return c, err
	}
	if len(c.items) == 0 {
		return c, errors.New("empty gift catalog")
	}
	seen := map[string]bool{}
	for _, s := range c.items {
		if !codePattern.MatchString(s.Code) || seen[s.Code] || strings.TrimSpace(s.Name) == "" || s.Price <= 0 || s.Price > math.MaxInt64/99 || effects[s.EffectType] == nil || s.Pin < 0 || s.Pin > math.MaxInt32 || s.Super != (s.EffectType == "superchat") || s.Super != (s.Pin > 0) {
			return c, fmt.Errorf("invalid gift spec %q", s.Code)
		}
		seen[s.Code] = true
	}
	return c, nil
}

type GiftRequest struct {
	SenderID uint64
	Quantity int
	Message  string
}
type GiftEffect interface {
	Type() string
	Prepare(GiftSpec, GiftRequest) (comboKey string, err error)
}
type EffectRegistry map[string]GiftEffect

func (r EffectRegistry) Register(e GiftEffect) error {
	if e == nil || e.Type() == "" || r[e.Type()] != nil {
		return errors.New("invalid or duplicate gift effect")
	}
	r[e.Type()] = e
	return nil
}

type standard struct{}

func (standard) Type() string { return "standard" }
func (standard) Prepare(_ GiftSpec, r GiftRequest) (string, error) {
	if r.Quantity < 1 || r.Quantity > 99 || utf8.RuneCountInString(r.Message) > 60 || strings.ContainsRune(r.Message, 0) {
		return "", errors.New("invalid gift request")
	}
	return "", nil
}

type superchat struct{}

func (superchat) Type() string { return "superchat" }
func (superchat) Prepare(_ GiftSpec, r GiftRequest) (string, error) {
	if r.Quantity != 1 || strings.TrimSpace(r.Message) == "" || utf8.RuneCountInString(r.Message) > 120 || strings.ContainsRune(r.Message, 0) {
		return "", errors.New("invalid superchat request")
	}
	return "", nil
}

type combo struct{}

func (combo) Type() string { return "combo" }
func (combo) Prepare(s GiftSpec, r GiftRequest) (string, error) {
	if _, err := (standard{}).Prepare(s, r); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%s", r.SenderID, s.Code), nil
}
func DefaultEffects() EffectRegistry {
	return EffectRegistry{"standard": standard{}, "combo": combo{}, "superchat": superchat{}}
}
