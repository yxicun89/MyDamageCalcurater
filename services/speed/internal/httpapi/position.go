package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"example.com/pokecalc/services/speed/internal/api"
	"example.com/pokecalc/services/speed/internal/speed"
	"github.com/labstack/echo/v5"
)

// errInvalidPositionBody は body が mode の型(ADR-0602 §2)に合わないことを表す。生成コードは
// body の中身を検証しないので、mode ごとの必須・余計なフィールド・範囲の検査はここで自前に行う。
var errInvalidPositionBody = errors.New("request body does not match the given mode")

// errPositionBodyTooLarge は body が maxPositionBodyBytes を超えたことを表す(balance の
// errRequestTooLarge と同じ形)。
var errPositionBodyTooLarge = errors.New("request body exceeds the size limit")

// getSpeedPosition implements POST /api/speed/v1/position (ADR-0602 §4): body(400。サイズ超過は
// 413)→ read model absent(503)→ unknown pokemonId(422)→ 200. Header の検査は
// requireRequestContext ミドルウェア(New で登録)が body より先に行う。
func getSpeedPosition(c *echo.Context, deps Dependencies) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxPositionBodyBytes)
	body, err := decodeJSONBody[api.PositionRequest](c.Request())
	if err != nil {
		if errors.Is(err, errPositionBodyTooLarge) {
			return c.JSON(http.StatusRequestEntityTooLarge, api.Error{
				Code:    api.RequestTooLarge,
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusBadRequest, api.Error{
			Code:    api.InvalidRequest,
			Message: err.Error(),
		})
	}

	req, err := toPositionRequest(body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Error{
			Code:    api.InvalidRequest,
			Message: err.Error(),
		})
	}

	if deps.Pokemon == nil {
		return c.JSON(http.StatusServiceUnavailable, api.Error{
			Code:    api.MasterUnavailable,
			Message: "the pokemon read model is not configured",
		})
	}
	roster, err := deps.Pokemon.Roster()
	if err != nil {
		return internalError(c, err)
	}

	result, err := speed.Position(roster, req)
	if err != nil {
		if errors.Is(err, speed.ErrUnknownPokemon) {
			return c.JSON(http.StatusUnprocessableEntity, api.Error{
				Code:    api.UnknownPokemon,
				Message: "pokemonId is not in the read model",
			})
		}
		// req はここまでに自前で検証済みなので、残るのは表の組み立ての想定外のエラー(read model が
		// 不正な場合など)だけ。他のハンドラと同じく 500 の固定文言にする(ADR-0602 §4)。
		return internalError(c, err)
	}

	return c.JSON(http.StatusOK, toPositionResponse(result))
}

// decodeJSONBody は body を厳密な JSON として T に詰め替える(balance の decodeJSONBody と同じ形。
// 未知フィールドは拒否、1 つの JSON 値だけを許す、http.MaxBytesReader が既に設定した上限を超えたら
// errPositionBodyTooLarge)。生成コードは body の形を検証しないため、speed 側で新設する
// (ADR-0602 §2 の申し送り)。
func decodeJSONBody[T any](request *http.Request) (T, error) {
	var body T
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return body, errPositionBodyTooLarge
		}
		return body, errors.New("request body must be a single valid JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return body, errPositionBodyTooLarge
		}
		return body, errors.New("request body must contain exactly one JSON object")
	}
	return body, nil
}

// toPositionRequest は生成型の PositionRequest を mode ごとの必須・余計なフィールドと範囲について
// 検証し、コアの speed.PositionRequest に詰め替える(ADR-0602 §2)。read model を使わずに検証できる
// ものはここで済ませる(read model 未設定でも 503 ではなく 400 を返すため)。
func toPositionRequest(body api.PositionRequest) (speed.PositionRequest, error) {
	switch body.Mode {
	case api.Preset:
		return positionRequestFromPreset(body)
	case api.Custom:
		return positionRequestFromCustom(body)
	case api.Raw:
		return positionRequestFromRaw(body)
	default:
		return speed.PositionRequest{}, errInvalidPositionBody
	}
}

// positionRequestFromPreset requires exactly pokemonId, preset and scarf (ADR-0602 §2). preset must
// be one of the 3 minimal presets (api.MinimalPresetId's enum, generated from openapi.yaml, is the
// single source for that list — not copied here).
func positionRequestFromPreset(body api.PositionRequest) (speed.PositionRequest, error) {
	if body.PokemonId == nil || body.Preset == nil || body.Scarf == nil {
		return speed.PositionRequest{}, errInvalidPositionBody
	}
	if body.Sp != nil || body.Nature != nil || body.Rank != nil || body.Value != nil {
		return speed.PositionRequest{}, errInvalidPositionBody
	}
	if !pokemonIDPattern.MatchString(*body.PokemonId) {
		return speed.PositionRequest{}, errInvalidPositionBody
	}
	if !body.Preset.Valid() {
		return speed.PositionRequest{}, errInvalidPositionBody
	}
	return speed.PositionRequest{
		Mode:      speed.PositionModePreset,
		PokemonID: *body.PokemonId,
		Preset:    speed.PresetID(*body.Preset),
		Scarf:     *body.Scarf,
	}, nil
}

// positionRequestFromCustom requires exactly pokemonId, sp, nature, rank and scarf (ADR-0602 §2). sp,
// nature and rank are range-checked by calling speed.Speed with a valid dummy base speed: that reuses
// the core's own sentinel checks instead of copying the SP/rank bounds as magic numbers here.
func positionRequestFromCustom(body api.PositionRequest) (speed.PositionRequest, error) {
	if body.PokemonId == nil || body.Sp == nil || body.Nature == nil || body.Rank == nil || body.Scarf == nil {
		return speed.PositionRequest{}, errInvalidPositionBody
	}
	if body.Preset != nil || body.Value != nil {
		return speed.PositionRequest{}, errInvalidPositionBody
	}
	if !pokemonIDPattern.MatchString(*body.PokemonId) {
		return speed.PositionRequest{}, errInvalidPositionBody
	}

	nature := speed.NatureEffect(*body.Nature)
	if _, err := speed.Speed(speed.Input{BaseSpeed: 1, SP: *body.Sp, Nature: nature, Rank: *body.Rank, Scarf: false}); err != nil {
		return speed.PositionRequest{}, errInvalidPositionBody
	}

	return speed.PositionRequest{
		Mode:      speed.PositionModeCustom,
		PokemonID: *body.PokemonId,
		SP:        *body.Sp,
		Nature:    nature,
		Rank:      *body.Rank,
		Scarf:     *body.Scarf,
	}, nil
}

// positionRequestFromRaw requires exactly value; pokemonId is optional, for display only (ADR-0602
// §2). value is range-checked against speed.RawSpeedRange, derived from the formula, not hardcoded.
func positionRequestFromRaw(body api.PositionRequest) (speed.PositionRequest, error) {
	if body.Value == nil {
		return speed.PositionRequest{}, errInvalidPositionBody
	}
	if body.Preset != nil || body.Scarf != nil || body.Sp != nil || body.Nature != nil || body.Rank != nil {
		return speed.PositionRequest{}, errInvalidPositionBody
	}

	minSpeed, maxSpeed := speed.RawSpeedRange()
	if *body.Value < minSpeed || *body.Value > maxSpeed {
		return speed.PositionRequest{}, errInvalidPositionBody
	}

	req := speed.PositionRequest{Mode: speed.PositionModeRaw, Value: *body.Value}
	if body.PokemonId != nil {
		if !pokemonIDPattern.MatchString(*body.PokemonId) {
			return speed.PositionRequest{}, errInvalidPositionBody
		}
		req.PokemonID = *body.PokemonId
	}
	return req, nil
}

// toPositionResponse converts speed.PositionResult to the generated api.PositionResponse. Tie is
// always a non-nil slice (possibly empty) so it serializes as [] rather than null (ADR-0602 §3).
func toPositionResponse(result speed.PositionResult) api.PositionResponse {
	tie := make([]api.SpeedTableEntry, len(result.Tie))
	for i, e := range result.Tie {
		types := make([]string, len(e.Pokemon.Types))
		copy(types, e.Pokemon.Types)
		tie[i] = api.SpeedTableEntry{
			PokemonId: e.Pokemon.PokemonID,
			NameJa:    e.Pokemon.NameJa,
			Types:     types,
			BaseSpeed: e.Pokemon.BaseSpeed,
			Preset:    api.PresetId(e.Preset),
		}
	}

	response := api.PositionResponse{
		Speed:  result.Speed,
		Faster: result.Faster,
		Slower: result.Slower,
		Tie:    tie,
	}
	if result.Pokemon != nil {
		types := make([]string, len(result.Pokemon.Types))
		copy(types, result.Pokemon.Types)
		response.Pokemon = &api.SpeedPokemon{
			PokemonId: result.Pokemon.PokemonID,
			NameJa:    result.Pokemon.NameJa,
			Types:     types,
			BaseSpeed: result.Pokemon.BaseSpeed,
		}
	}
	return response
}
