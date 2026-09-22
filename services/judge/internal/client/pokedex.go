package client

import (
	"context"
	"fmt"
	"net/http"
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/pokedex/species/"+key, nil)
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
