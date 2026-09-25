package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// Species is judge's own copy of the pokedex fields it reads (ADR-0700 §4): the field
// meaning is defined by SpeciesDetail in the root api/openapi.yaml, not here.
type Species struct {
	Key       string
	NameJa    string
	Types     []string
	BaseStats StatBlock
}

// Pokedex is a client to pokedex-svc's public API.
type Pokedex struct {
	baseURL string
	http    *http.Client
}

// NewPokedex validates config and builds a Pokedex client (ADR-0700 §3).
func NewPokedex(config Config) (*Pokedex, error) {
	baseURL, httpClient, err := buildHTTPClient(config)
	if err != nil {
		return nil, err
	}
	return &Pokedex{baseURL: baseURL, http: httpClient}, nil
}

// Species calls GET {base}/api/pokedex/species/{key} and extracts the fields judge reads
// (base stats and types). ctx and the configured Timeout race; whichever ends first wins
// (ADR-0700 §2).
func (p *Pokedex) Species(ctx context.Context, rc RequestContext, key string) (Species, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/pokedex/species/"+url.PathEscape(key), nil)
	if err != nil {
		return Species{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	resp, err := send(ctx, p.http, req, rc)
	if err != nil {
		return Species{}, err
	}

	var wire speciesWire
	if err := decodeUpstreamJSON(resp, &wire); err != nil {
		return Species{}, err
	}
	return wire.toSpecies()
}

// speciesWire mirrors just the fields judge reads from SpeciesDetail. baseStats and its
// six stats are pointers so a present-but-zero stat (e.g. spe: 0) is never confused with a
// field the upstream left out; silently defaulting a missing stat to 0 would quietly wreck
// a judgement (ADR-0700 テストの期待値).
type speciesWire struct {
	Key       string         `json:"key"`
	NameJa    string         `json:"nameJa"`
	Types     []string       `json:"types"`
	BaseStats *statBlockWire `json:"baseStats"`
}

type statBlockWire struct {
	HP  *int `json:"hp"`
	Atk *int `json:"atk"`
	Def *int `json:"def"`
	SpA *int `json:"spa"`
	SpD *int `json:"spd"`
	Spe *int `json:"spe"`
}

func (w speciesWire) toSpecies() (Species, error) {
	if len(w.Types) == 0 {
		return Species{}, fmt.Errorf("%w: species response has no types", ErrUpstreamInvalidResponse)
	}
	if w.BaseStats == nil {
		return Species{}, fmt.Errorf("%w: species response has no baseStats", ErrUpstreamInvalidResponse)
	}
	stats, err := w.BaseStats.toStatBlock()
	if err != nil {
		return Species{}, err
	}
	return Species{Key: w.Key, NameJa: w.NameJa, Types: w.Types, BaseStats: stats}, nil
}

func (w statBlockWire) toStatBlock() (StatBlock, error) {
	if w.HP == nil || w.Atk == nil || w.Def == nil || w.SpA == nil || w.SpD == nil || w.Spe == nil {
		return StatBlock{}, fmt.Errorf("%w: baseStats is missing a required stat", ErrUpstreamInvalidResponse)
	}
	return StatBlock{HP: *w.HP, Atk: *w.Atk, Def: *w.Def, SpA: *w.SpA, SpD: *w.SpD, Spe: *w.Spe}, nil
}

// --- JD1: 性格の解決(ADR-0701 §4) ---------------------------------------------------

// Nature is judge's own copy of the pokedex fields it reads for a nature: the field meaning
// is defined by Nature in the root api/openapi.yaml, not here. Plus/Minus stay plain strings
// (one of the six stat keys, or "" for a neutral nature) so this package stays independent of
// engine (ADR-0701 §4); internal/judge converts them to engine.StatKey.
type Nature struct {
	ID     string
	NameJa string
	Plus   string
	Minus  string
}

// natureStatKeys is the set of valid Nature.plus/minus values (the six stat keys). It exists
// only to validate the upstream response shape, not as a master list (ADR-0701 §4 rejects
// hardcoding a nature-name table; this is the fixed StatKey vocabulary, same footing as
// ADR-0600 §3's scarfSpeedModifier constant).
var natureStatKeys = map[string]bool{
	"hp": true, "atk": true, "def": true, "spa": true, "spd": true, "spe": true,
}

// Natures calls GET {base}/api/pokedex/natures and extracts the fields judge reads (ADR-0701
// §4). judge calls this once per client request and does not cache the result.
func (p *Pokedex) Natures(ctx context.Context, rc RequestContext) ([]Nature, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/pokedex/natures", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	resp, err := send(ctx, p.http, req, rc)
	if err != nil {
		return nil, err
	}

	var wire []natureWire
	if err := decodeUpstreamJSON(resp, &wire); err != nil {
		return nil, err
	}
	// An empty list is a contract violation, not "no natures": silently proceeding would let a
	// judgement resolve every natureId as unknown, or (worse) a caller mistake it for "no
	// correction" (ADR-0701 §4).
	if len(wire) == 0 {
		return nil, fmt.Errorf("%w: nature list is empty", ErrUpstreamInvalidResponse)
	}

	natures := make([]Nature, len(wire))
	for i, w := range wire {
		nature, err := w.toNature()
		if err != nil {
			return nil, err
		}
		natures[i] = nature
	}
	return natures, nil
}

// natureWire mirrors just the fields judge reads from the root api/openapi.yaml Nature. plus
// and minus are pointers so JSON null (a neutral nature) is distinguishable from an upstream
// bug that omits the field entirely; both end up as "" via normalizeNatureStat, but only the
// former is contractually valid.
type natureWire struct {
	ID     string  `json:"id"`
	NameJa string  `json:"nameJa"`
	Plus   *string `json:"plus"`
	Minus  *string `json:"minus"`
}

func (w natureWire) toNature() (Nature, error) {
	if w.ID == "" {
		return Nature{}, fmt.Errorf("%w: nature is missing id", ErrUpstreamInvalidResponse)
	}
	plus, err := normalizeNatureStat(w.Plus)
	if err != nil {
		return Nature{}, err
	}
	minus, err := normalizeNatureStat(w.Minus)
	if err != nil {
		return Nature{}, err
	}
	return Nature{ID: w.ID, NameJa: w.NameJa, Plus: plus, Minus: minus}, nil
}

// normalizeNatureStat maps a null/absent plus-or-minus to "" (no correction) and rejects
// anything that isn't one of the six stat keys (ADR-0701 §4: never silently fall back to
// neutral for a value the contract doesn't allow).
func normalizeNatureStat(v *string) (string, error) {
	if v == nil || *v == "" {
		return "", nil
	}
	if !natureStatKeys[*v] {
		return "", fmt.Errorf("%w: nature stat %q is not one of the six stats", ErrUpstreamInvalidResponse, *v)
	}
	return *v, nil
}

// --- JD4: 技の優先度の取得(ADR-0704 §9) ---------------------------------------------

// Move is judge's own copy of the pokedex fields it reads for a move: the field meaning is
// defined by Move in the root api/openapi.yaml, not here. judge reads only id and priority
// (power/type/category aren't needed; calc-svc computes damage).
type Move struct {
	ID       string
	Priority int
}

// Move calls GET {base}/api/pokedex/moves/{key} and extracts the fields judge reads (id and
// priority; ADR-0704 §9).
func (p *Pokedex) Move(ctx context.Context, rc RequestContext, key string) (Move, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/pokedex/moves/"+url.PathEscape(key), nil)
	if err != nil {
		return Move{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	resp, err := send(ctx, p.http, req, rc)
	if err != nil {
		return Move{}, err
	}

	var wire moveWire
	if err := decodeUpstreamJSON(resp, &wire); err != nil {
		return Move{}, err
	}
	return wire.toMove()
}

// moveWire mirrors just the fields judge reads from Move. Priority is a pointer so a
// present-but-zero priority (a normal move) is never confused with a field the upstream left
// out: silently defaulting a missing priority to 0 would make a priority move judge as a
// normal one, with a correct-looking result (ADR-0704 §9).
type moveWire struct {
	ID       string `json:"id"`
	Priority *int   `json:"priority"`
}

func (w moveWire) toMove() (Move, error) {
	if w.ID == "" {
		return Move{}, fmt.Errorf("%w: move response has no id", ErrUpstreamInvalidResponse)
	}
	if w.Priority == nil {
		return Move{}, fmt.Errorf("%w: move response has no priority", ErrUpstreamInvalidResponse)
	}
	return Move{ID: w.ID, Priority: *w.Priority}, nil
}
