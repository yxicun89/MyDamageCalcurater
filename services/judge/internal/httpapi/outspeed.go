package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/judge/internal/api"
	"example.com/pokecalc/services/judge/internal/client"
	"example.com/pokecalc/services/judge/internal/judge"
	"github.com/labstack/echo/v5"
)

// maxOutspeedBodyBytes: OutspeedAndKoRequest の妥当な body は 2 個体ぶんのスカラー欄程度で、
// 数百バイトに満たない。8 KiB は契約(api/openapi.yaml)が定める上限そのもの(ADR-0701 §5・§6)。
const maxOutspeedBodyBytes = 8 * 1024

// errOutspeedBodyTooLarge は body が maxOutspeedBodyBytes を超えたことを表す
// (services/speed の errPositionBodyTooLarge と同じ形)。
var errOutspeedBodyTooLarge = errors.New("request body exceeds the size limit")

// errInvalidOutspeedBody は body が契約の形・範囲に合わないことを表す。生成コードは JSON Schema
// の required/enum/min/max を検証しないので、httpapi 側で自前に検証する
// (services/speed の errInvalidPositionBody と同じ事情)。
var errInvalidOutspeedBody = errors.New("request body does not match the outspeed-and-ko contract")

// speciesKeyPattern is api/openapi.yaml's SpeciesKey pattern (judge holds it once, matching
// services/speed's pokemonIDPattern precedent: the generated code doesn't validate it).
var speciesKeyPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{3}$`)

// outspeedRequestWire mirrors OutspeedAndKoRequest with every field a pointer so a missing key
// is distinguishable from an explicit zero value (e.g. sp.spe: 0 is a legitimate "no
// investment", but a missing sp must still be rejected). Same reasoning as
// internal/client's speciesWire/statBlockWire.
type outspeedRequestWire struct {
	Format   *string         `json:"format"`
	Attacker *individualWire `json:"attacker"`
	Defender *individualWire `json:"defender"`
	MoveID   *string         `json:"moveId"`
	Field    *api.FieldState `json:"field"`
}

type individualWire struct {
	SpeciesKey *string        `json:"speciesKey"`
	NatureID   *string        `json:"natureId"`
	SP         *statBlockWire `json:"sp"`
	Ranks      *rankBlockWire `json:"ranks"`
	AbilityID  *string        `json:"abilityId"`
	ItemID     *string        `json:"itemId"`
}

type statBlockWire struct {
	HP  *int `json:"hp"`
	Atk *int `json:"atk"`
	Def *int `json:"def"`
	SpA *int `json:"spa"`
	SpD *int `json:"spd"`
	Spe *int `json:"spe"`
}

type rankBlockWire struct {
	Atk *int `json:"atk"`
	Def *int `json:"def"`
	SpA *int `json:"spa"`
	SpD *int `json:"spd"`
	Spe *int `json:"spe"`
}

// outspeedRequest is the validated, range-checked request (ADR-0701 §5: everything here is
// confirmed without calling an upstream).
type outspeedRequest struct {
	format   string
	attacker individualInput
	defender individualInput
	moveID   string
	field    *api.FieldState
}

type individualInput struct {
	speciesKey string
	natureID   string
	sp         engine.Stats
	ranks      engine.Ranks
	abilityID  string
	itemID     string
}

// outspeedAndKo implements POST /api/judge/v1/outspeed-and-ko, in the fixed check order ADR-0701
// §5 requires (also documented in api/openapi.yaml): header (middleware, ahead of this) → body
// shape/size → sp/ranks/format/required-string range (no upstream call yet) → natures (once) →
// unknown natureId → attacker species → defender species → calc → 200.
func outspeedAndKo(c *echo.Context, deps Dependencies, params api.OutspeedAndKoParams) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxOutspeedBodyBytes)
	wire, err := decodeOutspeedBody(c.Request())
	if err != nil {
		if errors.Is(err, errOutspeedBodyTooLarge) {
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

	req, err := toOutspeedRequest(wire)
	if err != nil {
		return c.JSON(http.StatusBadRequest, api.Error{
			Code:    api.InvalidRequest,
			Message: err.Error(),
		})
	}

	if deps.Pokedex == nil || deps.Calc == nil {
		return c.JSON(http.StatusServiceUnavailable, api.Error{
			Code:    api.UpstreamUnavailable,
			Message: "pokedex-svc/calc-svc is not configured",
		})
	}

	ctx := c.Request().Context()
	rc := client.RequestContext{DeviceID: params.XDeviceId, SessionID: params.XSessionId}

	natures, err := deps.Pokedex.Natures(ctx, rc)
	if err != nil {
		return writeUpstreamError(c, err)
	}
	natureTable := judge.NewNatureTable(toJudgeNatures(natures))

	attackerNature, err := natureTable.Lookup(req.attacker.natureID)
	if err != nil {
		return writeUnknownNature(c)
	}
	defenderNature, err := natureTable.Lookup(req.defender.natureID)
	if err != nil {
		return writeUnknownNature(c)
	}

	attackerSpecies, err := deps.Pokedex.Species(ctx, rc, req.attacker.speciesKey)
	if err != nil {
		return writeSpeciesError(c, err)
	}
	defenderSpecies, err := deps.Pokedex.Species(ctx, rc, req.defender.speciesKey)
	if err != nil {
		return writeSpeciesError(c, err)
	}

	comparison, err := judge.CompareSpeed(
		judge.Individual{
			BaseSpeed: attackerSpecies.BaseStats.Spe,
			Nature:    attackerNature,
			SP:        req.attacker.sp,
			Ranks:     req.attacker.ranks,
			Scarf:     judge.IsChoiceScarf(req.attacker.itemID, deps.ChoiceScarfItemID),
		},
		judge.Individual{
			BaseSpeed: defenderSpecies.BaseStats.Spe,
			Nature:    defenderNature,
			SP:        req.defender.sp,
			Ranks:     req.defender.ranks,
			Scarf:     judge.IsChoiceScarf(req.defender.itemID, deps.ChoiceScarfItemID),
		},
	)
	if err != nil {
		// req はここまでに自前で範囲を検証済みなので、残るのは pokedex-svc が契約に反する
		// 種族値(例えば spe: 0)を返した場合だけ(想定外)。
		return internalError(c, err)
	}

	result, err := deps.Calc.Damage(ctx, rc, client.CalcRequest{
		Format:   req.format,
		Attacker: toClientIndividual(req.attacker),
		Defender: toClientIndividual(req.defender),
		MoveID:   req.moveID,
		Field:    toClientField(req.field),
	})
	if err != nil {
		return writeCalcError(c, err)
	}

	return c.JSON(http.StatusOK, api.OutspeedAndKoResponse{
		Outspeeds:     comparison.Outspeeds,
		SpeedTie:      comparison.SpeedTie,
		AttackerSpeed: comparison.AttackerSpeed,
		DefenderSpeed: comparison.DefenderSpeed,
		Ko: api.KOChance{
			Hits:                 result.KO.Hits,
			Guaranteed:           result.KO.Guaranteed,
			DisplayChancePercent: result.KO.DisplayChancePercent,
		},
	})
}

// decodeOutspeedBody は body を厳密な JSON として outspeedRequestWire に詰め替える
// (services/speed の decodeJSONBody と同じ形: 未知フィールドは拒否、1 つの JSON 値だけを許す、
// http.MaxBytesReader が既に設定した上限を超えたら errOutspeedBodyTooLarge)。
func decodeOutspeedBody(request *http.Request) (outspeedRequestWire, error) {
	var body outspeedRequestWire
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return body, errOutspeedBodyTooLarge
		}
		return body, errors.New("request body must be a single valid JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return body, errOutspeedBodyTooLarge
		}
		return body, errors.New("request body must contain exactly one JSON object")
	}
	return body, nil
}

// toOutspeedRequest validates format/attacker/defender/moveId and the sp/ranks range of both
// individuals, all without calling an upstream (ADR-0701 §5).
func toOutspeedRequest(wire outspeedRequestWire) (outspeedRequest, error) {
	if wire.Format == nil || !api.Format(*wire.Format).Valid() {
		return outspeedRequest{}, errInvalidOutspeedBody
	}
	if wire.Attacker == nil || wire.Defender == nil {
		return outspeedRequest{}, errInvalidOutspeedBody
	}
	if wire.MoveID == nil || *wire.MoveID == "" {
		return outspeedRequest{}, errInvalidOutspeedBody
	}

	attacker, err := toIndividualInput(*wire.Attacker)
	if err != nil {
		return outspeedRequest{}, err
	}
	defender, err := toIndividualInput(*wire.Defender)
	if err != nil {
		return outspeedRequest{}, err
	}

	return outspeedRequest{
		format:   *wire.Format,
		attacker: attacker,
		defender: defender,
		moveID:   *wire.MoveID,
		field:    wire.Field,
	}, nil
}

// toIndividualInput validates one Individual's required strings and sp/ranks range. The
// sp/ranks range check reuses judge.Speed's own sentinel checks with a dummy valid BaseSpeed/
// Nature (same trick as services/speed's positionRequestFromCustom): judge doesn't copy the
// SP/rank bounds as magic numbers here.
func toIndividualInput(wire individualWire) (individualInput, error) {
	if wire.SpeciesKey == nil || !speciesKeyPattern.MatchString(*wire.SpeciesKey) {
		return individualInput{}, errInvalidOutspeedBody
	}
	if wire.NatureID == nil || *wire.NatureID == "" {
		return individualInput{}, errInvalidOutspeedBody
	}

	sp, err := toStats(wire.SP)
	if err != nil {
		return individualInput{}, err
	}
	ranks := toRanks(wire.Ranks)

	if _, err := judge.Speed(judge.Individual{BaseSpeed: 1, Nature: engine.NatureNeutral, SP: sp, Ranks: ranks}); err != nil {
		return individualInput{}, errInvalidOutspeedBody
	}

	input := individualInput{
		speciesKey: *wire.SpeciesKey,
		natureID:   *wire.NatureID,
		sp:         sp,
		ranks:      ranks,
	}
	if wire.AbilityID != nil {
		input.abilityID = *wire.AbilityID
	}
	if wire.ItemID != nil {
		input.itemID = *wire.ItemID
	}
	return input, nil
}

// toStats requires all 6 stats to be present (StatBlock is required in the contract); a
// missing stat is a shape error, not a 0 value (ADR-0701 §5).
func toStats(wire *statBlockWire) (engine.Stats, error) {
	if wire == nil {
		return engine.Stats{}, errInvalidOutspeedBody
	}
	if wire.HP == nil || wire.Atk == nil || wire.Def == nil || wire.SpA == nil || wire.SpD == nil || wire.Spe == nil {
		return engine.Stats{}, errInvalidOutspeedBody
	}
	return engine.Stats{HP: *wire.HP, Atk: *wire.Atk, Def: *wire.Def, SpA: *wire.SpA, SpD: *wire.SpD, Spe: *wire.Spe}, nil
}

// toRanks defaults every omitted rank to 0 (RankBlock's fields all default to 0 in the
// contract).
func toRanks(wire *rankBlockWire) engine.Ranks {
	if wire == nil {
		return engine.Ranks{}
	}
	ranks := engine.Ranks{}
	if wire.Atk != nil {
		ranks.Atk = *wire.Atk
	}
	if wire.Def != nil {
		ranks.Def = *wire.Def
	}
	if wire.SpA != nil {
		ranks.SpA = *wire.SpA
	}
	if wire.SpD != nil {
		ranks.SpD = *wire.SpD
	}
	if wire.Spe != nil {
		ranks.Spe = *wire.Spe
	}
	return ranks
}

// toJudgeNatures converts client.Nature (plain strings; ADR-0701 §4) to judge.Nature
// (engine.StatKey). client.Pokedex.Natures already rejects any plus/minus that isn't one of
// the six stat keys, so this conversion cannot fail.
func toJudgeNatures(natures []client.Nature) []judge.Nature {
	out := make([]judge.Nature, len(natures))
	for i, n := range natures {
		out[i] = judge.Nature{ID: n.ID, Plus: engine.StatKey(n.Plus), Minus: engine.StatKey(n.Minus)}
	}
	return out
}

// toClientIndividual builds the Individual judge sends to calc-svc. abilityId/itemId are sent
// only when the request set them (empty string omits the field via client.Individual's
// omitempty tag), never null vs "unset" confusion (ADR-0700 §4).
func toClientIndividual(input individualInput) client.Individual {
	return client.Individual{
		SpeciesKey: input.speciesKey,
		NatureID:   input.natureID,
		AbilityID:  input.abilityID,
		ItemID:     input.itemID,
		SP: client.StatBlock{
			HP: input.sp.HP, Atk: input.sp.Atk, Def: input.sp.Def,
			SpA: input.sp.SpA, SpD: input.sp.SpD, Spe: input.sp.Spe,
		},
		Ranks: client.RankBlock{
			Atk: input.ranks.Atk, Def: input.ranks.Def,
			SpA: input.ranks.SpA, SpD: input.ranks.SpD, Spe: input.ranks.Spe,
		},
	}
}

// toClientField forwards field as-is to calc-svc; judge never interprets weather/terrain/
// screens (ADR-0701 §1). nil stays nil so calc-svc falls back to its own default instead of
// receiving an explicit "no weather, no terrain, no screens".
func toClientField(wire *api.FieldState) *client.FieldState {
	if wire == nil {
		return nil
	}
	field := &client.FieldState{}
	if wire.Weather != nil {
		field.Weather = string(*wire.Weather)
	}
	if wire.Terrain != nil {
		field.Terrain = string(*wire.Terrain)
	}
	field.AttackerScreens = toClientScreens(wire.AttackerScreens)
	field.DefenderScreens = toClientScreens(wire.DefenderScreens)
	return field
}

func toClientScreens(wire *api.Screens) *client.Screens {
	if wire == nil {
		return nil
	}
	screens := &client.Screens{}
	if wire.Reflect != nil {
		screens.Reflect = *wire.Reflect
	}
	if wire.LightScreen != nil {
		screens.LightScreen = *wire.LightScreen
	}
	if wire.AuroraVeil != nil {
		screens.AuroraVeil = *wire.AuroraVeil
	}
	return screens
}

// writeUnknownNature answers 422 unknown_nature (ADR-0701 §6): a natureId that isn't in the
// nature list, for either side.
func writeUnknownNature(c *echo.Context) error {
	return c.JSON(http.StatusUnprocessableEntity, api.Error{
		Code:    api.UnknownNature,
		Message: "natureId is not in the nature list",
	})
}

// writeSpeciesError maps a Pokedex.Species failure to the ADR-0701 §6 table: ErrNotFound is
// unknown_species (422); everything else (unreachable/timeout/5xx/invalid response, or a
// pokedex-svc 400 the contract doesn't expect at this point) folds into upstream_unavailable,
// same as writeUpstreamError.
func writeSpeciesError(c *echo.Context, err error) error {
	if errors.Is(err, client.ErrNotFound) {
		return c.JSON(http.StatusUnprocessableEntity, api.Error{
			Code:    api.UnknownSpecies,
			Message: "speciesKey is not in the pokedex master",
		})
	}
	return writeUpstreamError(c, err)
}

// writeCalcError maps a Calc.Damage failure to the ADR-0701 §6 table: calc-svc's own 400
// (unknown move/item/ability, SP over budget, ...) folds into invalid_request, because judge
// cannot distinguish which one it was without reading calc-svc's body (ADR-0700 §3). Everything
// else folds into upstream_unavailable.
func writeCalcError(c *echo.Context, err error) error {
	if errors.Is(err, client.ErrInvalidRequest) {
		return c.JSON(http.StatusBadRequest, api.Error{
			Code:    api.InvalidRequest,
			Message: "calc-svc did not accept the calculation request",
		})
	}
	return writeUpstreamError(c, err)
}

// writeUpstreamError answers 503 upstream_unavailable (ADR-0701 §6): pokedex-svc/calc-svc
// unreachable, timed out, 5xx, or a response that doesn't match the contract. The message never
// includes upstream detail (ADR-0700 §3); the actual err is not logged here since callers of
// this package already fold out any upstream body/URL.
func writeUpstreamError(c *echo.Context, _ error) error {
	return c.JSON(http.StatusServiceUnavailable, api.Error{
		Code:    api.UpstreamUnavailable,
		Message: "upstream is unavailable",
	})
}
