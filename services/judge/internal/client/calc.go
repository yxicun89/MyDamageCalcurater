package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// RankBlock is judge's copy of the root api/openapi.yaml RankBlock (no HP; -6..+6, default 0).
type RankBlock struct {
	Atk int `json:"atk"`
	Def int `json:"def"`
	SpA int `json:"spa"`
	SpD int `json:"spd"`
	Spe int `json:"spe"`
}

// Individual is judge's copy of the required fields of the root api/openapi.yaml Individual
// that it needs to send (ADR-0700 §4): the rest of that schema (level, teraType, status, ...)
// isn't something JD0 has a value for yet.
type Individual struct {
	SpeciesKey string    `json:"speciesKey"`
	NatureID   string    `json:"natureId"`
	AbilityID  string    `json:"abilityId,omitempty"`
	ItemID     string    `json:"itemId,omitempty"`
	SP         StatBlock `json:"sp"`
	Ranks      RankBlock `json:"ranks"`
}

// Screens is judge's copy of the root api/openapi.yaml Screens (ADR-0701 §1). judge does not
// interpret it; it's forwarded to calc-svc as-is. `omitempty` drops an unset (false) field from
// the wire instead of sending it explicitly; this is equivalent for calc-svc, whose own schema
// defaults each of these to false.
type Screens struct {
	Reflect     bool `json:"reflect,omitempty"`
	LightScreen bool `json:"lightScreen,omitempty"`
	AuroraVeil  bool `json:"auroraVeil,omitempty"`
}

// FieldState is judge's copy of the root api/openapi.yaml FieldState's fields that judge
// forwards to calc-svc (ADR-0701 §1). judge does not interpret weather/terrain/screens; a nil
// pointer means "not sent" (calc-svc's own default), never "explicitly cleared" (ADR-0700 §4).
type FieldState struct {
	Weather         string   `json:"weather,omitempty"`
	Terrain         string   `json:"terrain,omitempty"`
	AttackerScreens *Screens `json:"attackerScreens,omitempty"`
	DefenderScreens *Screens `json:"defenderScreens,omitempty"`
}

// CalcRequest is judge's copy of the root api/openapi.yaml CalcRequest's required fields.
type CalcRequest struct {
	Format   string      `json:"format"`
	Attacker Individual  `json:"attacker"`
	Defender Individual  `json:"defender"`
	MoveID   string      `json:"moveId"`
	Field    *FieldState `json:"field,omitempty"`
}

// KOChance is judge's copy of the fields it reads from the root api/openapi.yaml KOChance.
// judge never recomputes it; it's transcribed as calc-svc returns it (ADR-0006, ADR-0010 §3).
type KOChance struct {
	Hits                 int
	Guaranteed           bool
	DisplayChancePercent float64
}

// UnsupportedMark is judge's copy of the fields it reads from the root api/openapi.yaml
// UnsupportedMark (ADR-0123, ADR-0708 §7): target/reason/id, byte-for-byte, judge does not
// interpret them (ADR-0708 §4).
type UnsupportedMark struct {
	Target string
	Reason string
	ID     string
}

// CalcResult is judge's copy of the fields it reads from the root api/openapi.yaml CalcResult
// (ADR-0700 §4).
type CalcResult struct {
	MinDamage   int
	MaxDamage   int
	DefenderHP  int
	KO          KOChance
	Unsupported []UnsupportedMark
}

// Calc is a client to calc-svc's public API.
type Calc struct {
	baseURL string
	http    *http.Client
}

// NewCalc validates config and builds a Calc client (ADR-0700 §3).
func NewCalc(config Config) (*Calc, error) {
	baseURL, httpClient, err := buildHTTPClient(config)
	if err != nil {
		return nil, err
	}
	return &Calc{baseURL: baseURL, http: httpClient}, nil
}

// Damage calls POST {base}/api/calc with request and extracts the damage range and KO
// chance judge reads. ctx and the configured Timeout race; whichever ends first wins
// (ADR-0700 §2).
func (c *Calc) Damage(ctx context.Context, rc RequestContext, request CalcRequest) (CalcResult, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return CalcResult{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/calc", bytes.NewReader(body))
	if err != nil {
		return CalcResult{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := send(ctx, c.http, req, rc)
	if err != nil {
		return CalcResult{}, err
	}

	var wire calcResultWire
	if err := decodeUpstreamJSON(resp, &wire); err != nil {
		return CalcResult{}, err
	}
	return wire.toCalcResult()
}

// calcResultWire mirrors just the fields judge reads from CalcResult. Every field is a
// pointer so a legitimate 0 (e.g. minDamage: 0, a move that deals no damage) is never
// confused with a field the upstream left out (same reasoning as speciesWire).
//
// Unsupported is *[]unsupportedMarkWire (not []unsupportedMarkWire) so a missing "unsupported"
// key (nil pointer) is distinguishable from an explicit "unsupported": [] (non-nil pointer to
// an empty slice). ADR-0708 §7: calc-svc's contract makes unsupported required, so a missing key
// is ErrUpstreamInvalidResponse, not "no marks".
type calcResultWire struct {
	MinDamage   *int                   `json:"minDamage"`
	MaxDamage   *int                   `json:"maxDamage"`
	DefenderHP  *int                   `json:"defenderHP"`
	KO          *koChanceWire          `json:"ko"`
	Unsupported *[]unsupportedMarkWire `json:"unsupported"`
}

type koChanceWire struct {
	Hits                 *int     `json:"hits"`
	Guaranteed           *bool    `json:"guaranteed"`
	DisplayChancePercent *float64 `json:"displayChancePercent"`
}

// unsupportedMarkWire mirrors UnsupportedMark(ADR-0123・ADR-0708 §7)。target/reason/id は
// どれも欠けたら ErrUpstreamInvalidResponse(空文字列は「欠けている」と見なさない。§7)。
type unsupportedMarkWire struct {
	Target *string `json:"target"`
	Reason *string `json:"reason"`
	ID     *string `json:"id"`
}

func (w calcResultWire) toCalcResult() (CalcResult, error) {
	if w.MinDamage == nil || w.MaxDamage == nil || w.DefenderHP == nil {
		return CalcResult{}, fmt.Errorf("%w: calc response is missing minDamage/maxDamage/defenderHP", ErrUpstreamInvalidResponse)
	}
	if w.KO == nil {
		return CalcResult{}, fmt.Errorf("%w: calc response has no ko", ErrUpstreamInvalidResponse)
	}
	ko, err := w.KO.toKOChance()
	if err != nil {
		return CalcResult{}, err
	}
	if w.Unsupported == nil {
		return CalcResult{}, fmt.Errorf("%w: calc response has no unsupported", ErrUpstreamInvalidResponse)
	}
	unsupported, err := toUnsupportedMarks(*w.Unsupported)
	if err != nil {
		return CalcResult{}, err
	}
	return CalcResult{MinDamage: *w.MinDamage, MaxDamage: *w.MaxDamage, DefenderHP: *w.DefenderHP, KO: ko, Unsupported: unsupported}, nil
}

func (w koChanceWire) toKOChance() (KOChance, error) {
	if w.Hits == nil || w.Guaranteed == nil || w.DisplayChancePercent == nil {
		return KOChance{}, fmt.Errorf("%w: ko is missing hits/guaranteed/displayChancePercent", ErrUpstreamInvalidResponse)
	}
	return KOChance{Hits: *w.Hits, Guaranteed: *w.Guaranteed, DisplayChancePercent: *w.DisplayChancePercent}, nil
}

// toUnsupportedMarks converts calc-svc の unsupported をそのまま・同じ順で UnsupportedMark に
// 変換する(ADR-0708 §4: judge は解釈・並べ替え・間引きをしない)。印が無い([])なら**空スライス**
// を返す(nil にしない。ADR-0708 §3・§7: 契約の [] をそのまま実装が破らないようにする)。
func toUnsupportedMarks(wire []unsupportedMarkWire) ([]UnsupportedMark, error) {
	marks := make([]UnsupportedMark, 0, len(wire))
	for _, m := range wire {
		if m.Target == nil || m.Reason == nil || m.ID == nil {
			return nil, fmt.Errorf("%w: unsupported element is missing target/reason/id", ErrUpstreamInvalidResponse)
		}
		marks = append(marks, UnsupportedMark{Target: *m.Target, Reason: *m.Reason, ID: *m.ID})
	}
	return marks, nil
}
