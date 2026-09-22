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

// CalcRequest is judge's copy of the root api/openapi.yaml CalcRequest's required fields.
type CalcRequest struct {
	Format   string     `json:"format"`
	Attacker Individual `json:"attacker"`
	Defender Individual `json:"defender"`
	MoveID   string     `json:"moveId"`
}

// KOChance is judge's copy of the fields it reads from the root api/openapi.yaml KOChance.
// judge never recomputes it; it's transcribed as calc-svc returns it (ADR-0006, ADR-0010 §3).
type KOChance struct {
	Hits                 int
	Guaranteed           bool
	DisplayChancePercent float64
}

// CalcResult is judge's copy of the fields it reads from the root api/openapi.yaml CalcResult
// (ADR-0700 §4).
type CalcResult struct {
	MinDamage  int
	MaxDamage  int
	DefenderHP int
	KO         KOChance
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
type calcResultWire struct {
	MinDamage  *int          `json:"minDamage"`
	MaxDamage  *int          `json:"maxDamage"`
	DefenderHP *int          `json:"defenderHP"`
	KO         *koChanceWire `json:"ko"`
}

type koChanceWire struct {
	Hits                 *int     `json:"hits"`
	Guaranteed           *bool    `json:"guaranteed"`
	DisplayChancePercent *float64 `json:"displayChancePercent"`
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
	return CalcResult{MinDamage: *w.MinDamage, MaxDamage: *w.MaxDamage, DefenderHP: *w.DefenderHP, KO: ko}, nil
}

func (w koChanceWire) toKOChance() (KOChance, error) {
	if w.Hits == nil || w.Guaranteed == nil || w.DisplayChancePercent == nil {
		return KOChance{}, fmt.Errorf("%w: ko is missing hits/guaranteed/displayChancePercent", ErrUpstreamInvalidResponse)
	}
	return KOChance{Hits: *w.Hits, Guaranteed: *w.Guaranteed, DisplayChancePercent: *w.DisplayChancePercent}, nil
}
