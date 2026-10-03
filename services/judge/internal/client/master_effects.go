package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// MaxMasterExportBytes は内部 API のマスタ一式を読む本文の上限(4 MiB。calc-svc の MaxExportBytes と同じ)。
// 1 件の種族・技より大きいので、maxResponseBodyBytes とは別に持つ。
const MaxMasterExportBytes = 4 << 20

// MasterEffectEntry は内部 API のマスタの特性・持ち物 1 件のうち、judge が読む欄(ID と効果)。
// Effect は自由形式の JSON のまま。中身の検証は internal/speedeffects が ID ごとに行う。
type MasterEffectEntry struct {
	ID     string
	Effect json.RawMessage
}

// MasterEffects は内部 API のマスタから抜き出した特性・持ち物の効果。種族・技などは読まず保持しない
// (issue 235 第2段・ADR-0714 §1)。
type MasterEffects struct {
	Abilities []MasterEffectEntry
	Items     []MasterEffectEntry
}

// MasterEffects calls GET {base}/internal/pokedex/master and extracts the ability and item ids and
// effects judge reads. The internal API needs no device/session identity, and the one fetch is
// shared by every request, so none is sent. The status folding and the sentinel errors are the same
// as the other calls (ADR-0700 §3); a body whose overall shape breaks the contract is
// ErrUpstreamInvalidResponse (the table is never built partially).
func (p *Pokedex) MasterEffects(ctx context.Context) (MasterEffects, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/internal/pokedex/master", nil)
	if err != nil {
		return MasterEffects{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	resp, err := doRequest(ctx, p.http, req)
	if err != nil {
		return MasterEffects{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := readBodyUpTo(resp.Body, MaxMasterExportBytes)
	if err != nil {
		return MasterEffects{}, fmt.Errorf("%w: %v", ErrUpstreamInvalidResponse, err)
	}
	out, err := decodeMasterEffects(data)
	if err != nil {
		return MasterEffects{}, fmt.Errorf("%w: %v", ErrUpstreamInvalidResponse, err)
	}
	return out, nil
}

// masterEffectsWire mirrors just the two lists judge reads. A pointer to the slice tells a missing
// or null list apart from an empty one.
type masterEffectsWire struct {
	Items     *[]masterEffectEntryWire `json:"items"`
	Abilities *[]masterEffectEntryWire `json:"abilities"`
}

type masterEffectEntryWire struct {
	ID     *string         `json:"id"`
	Effect json.RawMessage `json:"effect"`
}

func decodeMasterEffects(data []byte) (MasterEffects, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	var wire masterEffectsWire
	if err := dec.Decode(&wire); err != nil {
		return MasterEffects{}, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return MasterEffects{}, errors.New("trailing data after the JSON document")
	}
	if wire.Items == nil || wire.Abilities == nil {
		return MasterEffects{}, errors.New("master export has no items or abilities")
	}
	items, err := toMasterEffectEntries(*wire.Items, "item")
	if err != nil {
		return MasterEffects{}, err
	}
	abilities, err := toMasterEffectEntries(*wire.Abilities, "ability")
	if err != nil {
		return MasterEffects{}, err
	}
	return MasterEffects{Abilities: abilities, Items: items}, nil
}

func toMasterEffectEntries(wire []masterEffectEntryWire, kind string) ([]MasterEffectEntry, error) {
	seen := make(map[string]bool, len(wire))
	out := make([]MasterEffectEntry, 0, len(wire))
	for _, w := range wire {
		if w.ID == nil || *w.ID == "" {
			return nil, fmt.Errorf("%s is missing id", kind)
		}
		if seen[*w.ID] {
			return nil, fmt.Errorf("%s id is duplicated", kind)
		}
		seen[*w.ID] = true
		out = append(out, MasterEffectEntry{ID: *w.ID, Effect: w.Effect})
	}
	return out, nil
}
