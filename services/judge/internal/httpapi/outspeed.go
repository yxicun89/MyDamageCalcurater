package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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

// moveIDPattern / natureIDPattern are api/openapi.yaml's MoveId / NatureId pattern (Showdown
// ID: lowercase alphanumeric, hyphen-separated). Both fields share the same regex (ADR-0706
// §1), matching services/balance's MoveId/AbilityId precedent (ADR-0016 §2・ADR-0017 §2). The
// length limit is checked separately via maxIDLength, not embedded in the regex, so the
// contract's maxLength and this pattern don't need to agree on the same number in two places
// (ADR-0706 §1).
var (
	moveIDPattern   = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	natureIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

// maxIDLength is the upper bound on moveId/natureId length (ADR-0706 §1: judge doesn't hold the
// master, so this exists only to keep an unbounded value from becoming a path element sent
// upstream, matching api/openapi.yaml's MoveId/NatureId maxLength).
const maxIDLength = 64

// minDefenders/maxDefenders: ADR-0703 §1 の defenders の件数の上下限。oapi-codegen の生成コードは
// minItems/maxItems を検証しないので、httpapi 側が自前で検査する。
const (
	minDefenders = 1
	maxDefenders = 6
)

// outspeedRequestWire mirrors OutspeedAndKoRequest with every field a pointer so a missing key
// is distinguishable from an explicit zero value (e.g. sp.spe: 0 is a legitimate "no
// investment", but a missing sp must still be rejected). Same reasoning as
// internal/client's speciesWire/statBlockWire.
//
// Defenders は JD3(ADR-0703 §1)で単数の defender を置き換えた欄。要素は raw JSON のまま持ち、
// parseCandidateWire が自前でキー集合を検査してから DefenderCandidate に変換する(JD4。下記)。
type outspeedRequestWire struct {
	Format     *string           `json:"format"`
	Attacker   json.RawMessage   `json:"attacker"`
	Defenders  []json.RawMessage `json:"defenders"`
	MoveID     *string           `json:"moveId"`
	Field      *api.FieldState   `json:"field"`
	SpeedField json.RawMessage   `json:"speedField"`
}

type individualWire struct {
	SpeciesKey *string        `json:"speciesKey"`
	NatureID   *string        `json:"natureId"`
	SP         *statBlockWire `json:"sp"`
	Ranks      *rankBlockWire `json:"ranks"`
	AbilityID  *string        `json:"abilityId"`
	ItemID     *string        `json:"itemId"`
}

// defenderCandidateWire mirrors DefenderCandidate(ADR-0704 §1): individualWire の全欄に加えて、
// この候補が撃ち返す技 moveId を持つ。MoveID は json.RawMessage で受け、型検査
// (文字列かどうか)は decodeRequiredString に任せる: *string にすると「数値が来た」場合に
// この構造体の Unmarshal 自体が失敗し、defenders[<index>] の帰属を失ってしまう(assertBlamesCandidate)。
type defenderCandidateWire struct {
	individualWire
	MoveID json.RawMessage `json:"moveId"`
}

// individualWireKeys is the exact (case-sensitive) allow-list for an Individual's own keys
// (attacker's and, via candidateWireKeys below, each defender candidate's), checked separately
// from decoder.DisallowUnknownFields(): encoding/json's per-object field matching falls back to
// a case-insensitive match for a key with no exact match (e.g. "specieskey" folding onto the
// "speciesKey" field), silently overwriting it instead of rejecting the typo. This applies at
// every nesting level of a single decoder.Decode call, not just the top level, so attacker needs
// the same allow-list as defenders (same concern as speedFieldKeys, ADR-0702 受け入れ条件7・
// ADR-0704 テストの期待値・plan.md の JD4/JD5 critic 指摘の積み残し)。
var individualWireKeys = map[string]bool{
	"speciesKey": true, "natureId": true, "sp": true, "ranks": true,
	"abilityId": true, "itemId": true,
}

// candidateWireKeys extends individualWireKeys with moveId (a defenders[] element is an
// Individual plus its own counter move. ADR-0704 §1).
var candidateWireKeys = func() map[string]bool {
	keys := make(map[string]bool, len(individualWireKeys)+1)
	for key := range individualWireKeys {
		keys[key] = true
	}
	keys["moveId"] = true
	return keys
}()

// parseIndividualWire validates an Individual's key set exactly (individualWireKeys) before
// decoding it, so a case-typo'd or unrelated key is rejected instead of silently accepted or
// silently overwriting a known field. Used for attacker; defenders[] elements go through
// parseCandidateWire (candidateWireKeys), which additionally allows moveId.
func parseIndividualWire(raw json.RawMessage) (individualWire, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return individualWire{}, errInvalidOutspeedBody
	}
	for key := range fields {
		if !individualWireKeys[key] {
			return individualWire{}, errInvalidOutspeedBody
		}
	}
	var wire individualWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return individualWire{}, errInvalidOutspeedBody
	}
	return wire, nil
}

// parseCandidateWire validates one defenders[] element's key set exactly (candidateWireKeys)
// before decoding it into defenderCandidateWire, so a case-typo'd or unrelated key is rejected
// instead of silently accepted or silently overwriting a known field.
func parseCandidateWire(raw json.RawMessage) (defenderCandidateWire, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return defenderCandidateWire{}, errInvalidOutspeedBody
	}
	for key := range fields {
		if !candidateWireKeys[key] {
			return defenderCandidateWire{}, errInvalidOutspeedBody
		}
	}
	var wire defenderCandidateWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return defenderCandidateWire{}, errInvalidOutspeedBody
	}
	return wire, nil
}

// validMoveID / validNatureID check ADR-0706 §1's format (Showdown ID) and length
// (maxIDLength) for moveId / natureId. Both use the same regex, but are kept as separate
// functions matching speciesKeyPattern's call-site convention (one pattern per field kind).
func validMoveID(id string) bool {
	return len(id) <= maxIDLength && moveIDPattern.MatchString(id)
}

func validNatureID(id string) bool {
	return len(id) <= maxIDLength && natureIDPattern.MatchString(id)
}

// decodeRequiredString decodes raw as a required non-empty JSON string. A missing key (raw ==
// nil), an explicit null, an empty string, or a non-string value are all rejected.
func decodeRequiredString(raw json.RawMessage) (string, bool) {
	if raw == nil {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	if s == "" {
		return "", false
	}
	return s, true
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
// confirmed without calling an upstream). defenders は 1〜6 件(ADR-0703 §1)。
type outspeedRequest struct {
	format     string
	attacker   individualInput
	defenders  []defenderInput
	moveID     string
	field      *api.FieldState
	speedField speedFieldInput
}

// speedFieldInput is the resolved SpeedField (ADR-0702 §1): a missing speedField, or a missing
// field within it, is false (no field effect), matching JD1's behavior exactly.
type speedFieldInput struct {
	trickRoom        bool
	attackerTailwind bool
	defenderTailwind bool
}

type individualInput struct {
	speciesKey string
	natureID   string
	sp         engine.Stats
	ranks      engine.Ranks
	abilityID  string
	itemID     string
}

// defenderInput is one DefenderCandidate after validation (ADR-0704 §1): individualInput plus
// the move this candidate uses to strike back.
type defenderInput struct {
	individualInput
	moveID string
}

// outspeedAndKo implements POST /api/judge/v1/outspeed-and-ko, in the fixed check order ADR-0701
// §5 / ADR-0703 §4 / ADR-0704 §5 require (also documented in api/openapi.yaml): header
// (middleware, ahead of this) → body shape/size → defenders count (1..6) → sp/ranks/format/
// required-string range (attacker, then defenders index-ascending; no upstream call yet) →
// natures (once) → unknown natureId (attacker, then defenders index-ascending) → attacker
// species → attacker move → defenders species/move (per candidate: species then move,
// index-ascending) → calc (per candidate: forward then reverse, index-ascending) → 200. Any
// candidate failure stops the whole request (ADR-0703 §3: no partial success).
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
		return writeUnknownNature(c, "attacker")
	}

	defenderNatures := make([]engine.Nature, len(req.defenders))
	for i, defender := range req.defenders {
		nature, err := natureTable.Lookup(defender.natureID)
		if err != nil {
			return writeUnknownNature(c, candidateLabel(i))
		}
		defenderNatures[i] = nature
	}

	attackerSpecies, err := deps.Pokedex.Species(ctx, rc, req.attacker.speciesKey)
	if err != nil {
		return writeSpeciesError(c, err, "attacker")
	}

	attackerMove, err := deps.Pokedex.Move(ctx, rc, req.moveID)
	if err != nil {
		return writeMoveError(c, err, "attacker")
	}

	// 候補ごとに「種族 → 技」を続けて解決し、全候補が揃うまで calc へ進まない
	// (ADR-0704 §5: 同じ候補の中で続けてよいが、候補 0 の calc が候補 1 の種族より先に走ってはいけない)。
	defenderSpecies := make([]client.Species, len(req.defenders))
	defenderMoves := make([]client.Move, len(req.defenders))
	for i, defender := range req.defenders {
		species, err := deps.Pokedex.Species(ctx, rc, defender.speciesKey)
		if err != nil {
			return writeSpeciesError(c, err, candidateLabel(i))
		}
		defenderSpecies[i] = species

		move, err := deps.Pokedex.Move(ctx, rc, defender.moveID)
		if err != nil {
			return writeMoveError(c, err, candidateLabel(i))
		}
		defenderMoves[i] = move
	}

	matchups := make([]api.Matchup, len(req.defenders))
	for i, defender := range req.defenders {
		comparison, err := judge.CompareSpeed(
			judge.Individual{
				BaseSpeed: attackerSpecies.BaseStats.Spe,
				Nature:    attackerNature,
				SP:        req.attacker.sp,
				Ranks:     req.attacker.ranks,
				Scarf:     judge.IsChoiceScarf(req.attacker.itemID, deps.ChoiceScarfItemID),
				Tailwind:  req.speedField.attackerTailwind,
			},
			judge.Individual{
				BaseSpeed: defenderSpecies[i].BaseStats.Spe,
				Nature:    defenderNatures[i],
				SP:        defender.sp,
				Ranks:     defender.ranks,
				Scarf:     judge.IsChoiceScarf(defender.itemID, deps.ChoiceScarfItemID),
				Tailwind:  req.speedField.defenderTailwind,
			},
			judge.SpeedField{TrickRoom: req.speedField.trickRoom},
		)
		if err != nil {
			// req はここまでに自前で範囲を検証済みなので、残るのは pokedex-svc が契約に反する
			// 種族値(例えば spe: 0)を返した場合だけ(想定外)。
			return internalError(c, err)
		}

		turnOrder := judge.CompareTurnOrder(attackerMove.Priority, defenderMoves[i].Priority, comparison)

		// 順方向(自分 → この候補。ADR-0704 §4)。
		forward, err := deps.Calc.Damage(ctx, rc, client.CalcRequest{
			Format:   req.format,
			Attacker: toClientIndividual(req.attacker),
			Defender: toClientIndividual(defender.individualInput),
			MoveID:   req.moveID,
			Field:    toClientField(req.field),
		})
		if err != nil {
			return writeCalcError(c, err, candidateLabel(i))
		}

		// 逆方向(この候補 → 自分。役割が入れ替わるので attacker/defender/moveId/field の壁を
		// 入れ替えて送る。ADR-0704 §4)。
		reverse, err := deps.Calc.Damage(ctx, rc, client.CalcRequest{
			Format:   req.format,
			Attacker: toClientIndividual(defender.individualInput),
			Defender: toClientIndividual(req.attacker),
			MoveID:   defender.moveID,
			Field:    toReverseClientField(req.field),
		})
		if err != nil {
			return writeCalcError(c, err, candidateLabel(i))
		}

		matchups[i] = api.Matchup{
			DefenderIndex:        i,
			Outspeeds:            comparison.Outspeeds,
			SpeedTie:             comparison.SpeedTie,
			AttackerSpeed:        comparison.AttackerSpeed,
			DefenderSpeed:        comparison.DefenderSpeed,
			AttackerMovePriority: attackerMove.Priority,
			DefenderMovePriority: defenderMoves[i].Priority,
			AttackerMovesFirst:   turnOrder.AttackerMovesFirst,
			TurnOrderTie:         turnOrder.Tie,
			AttackerKo: api.KOChance{
				Hits:                 forward.KO.Hits,
				Guaranteed:           forward.KO.Guaranteed,
				DisplayChancePercent: forward.KO.DisplayChancePercent,
			},
			DefenderKo: api.KOChance{
				Hits:                 reverse.KO.Hits,
				Guaranteed:           reverse.KO.Guaranteed,
				DisplayChancePercent: reverse.KO.DisplayChancePercent,
			},
		}
	}

	return c.JSON(http.StatusOK, api.OutspeedAndKoResponse{Matchups: matchups})
}

// candidateLabel は ADR-0703 §3 の「どの候補で失敗したか」を示す message の断片。
func candidateLabel(index int) string {
	return fmt.Sprintf("defenders[%d]", index)
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

// toOutspeedRequest validates defenders count/format/attacker/moveId and the sp/ranks range of
// attacker and every defender, all without calling an upstream (ADR-0701 §5・ADR-0703 §4). The
// defenders count is checked before any range check (ADR-0703 §4: a 7th candidate wins over a
// range error in candidate 1), and range errors name attacker or the failing defenders[<index>]
// (ADR-0703 §3), attacker first, then defenders index-ascending, stopping at the first failure.
func toOutspeedRequest(wire outspeedRequestWire) (outspeedRequest, error) {
	if len(wire.Defenders) < minDefenders || len(wire.Defenders) > maxDefenders {
		return outspeedRequest{}, errInvalidOutspeedBody
	}
	if wire.Format == nil || !api.Format(*wire.Format).Valid() {
		return outspeedRequest{}, errInvalidOutspeedBody
	}
	if wire.Attacker == nil {
		return outspeedRequest{}, errInvalidOutspeedBody
	}
	if wire.MoveID == nil || *wire.MoveID == "" {
		return outspeedRequest{}, fmt.Errorf("%w: attacker", errInvalidOutspeedBody)
	}
	if !validMoveID(*wire.MoveID) {
		return outspeedRequest{}, fmt.Errorf("%w: attacker", errInvalidOutspeedBody)
	}

	attackerWire, err := parseIndividualWire(wire.Attacker)
	if err != nil {
		return outspeedRequest{}, fmt.Errorf("%w: attacker", errInvalidOutspeedBody)
	}
	attacker, err := toIndividualInput(attackerWire)
	if err != nil {
		return outspeedRequest{}, fmt.Errorf("%w: attacker", errInvalidOutspeedBody)
	}

	defenders := make([]defenderInput, len(wire.Defenders))
	for i, raw := range wire.Defenders {
		candidateWire, err := parseCandidateWire(raw)
		if err != nil {
			return outspeedRequest{}, fmt.Errorf("%w: %s", errInvalidOutspeedBody, candidateLabel(i))
		}
		defender, err := toDefenderInput(candidateWire)
		if err != nil {
			return outspeedRequest{}, fmt.Errorf("%w: %s", errInvalidOutspeedBody, candidateLabel(i))
		}
		defenders[i] = defender
	}

	speedField, err := toSpeedFieldInput(wire.SpeedField)
	if err != nil {
		return outspeedRequest{}, err
	}

	return outspeedRequest{
		format:     *wire.Format,
		attacker:   attacker,
		defenders:  defenders,
		moveID:     *wire.MoveID,
		field:      wire.Field,
		speedField: speedField,
	}, nil
}

// speedFieldKeys is the exact (case-sensitive) allow-list for speedField's own object, checked
// separately from the outer decoder.DisallowUnknownFields(): encoding/json falls back to a
// case-insensitive field match, which would otherwise silently accept a typo like "trickroom"
// (ADR-0702 受け入れ条件7).
var speedFieldKeys = map[string]bool{"trickRoom": true, "attackerTailwind": true, "defenderTailwind": true}

// toSpeedFieldInput はワイヤの speedField(3 欄すべて省略可)を厳密に検証して解決する。
// speedField 自体の省略・null・空オブジェクトも、個々の欄の省略も、すべて false
// (ADR-0702 §1・受け入れ条件5)。未知の欄・真偽値でない値は errInvalidOutspeedBody。
func toSpeedFieldInput(raw json.RawMessage) (speedFieldInput, error) {
	if raw == nil || string(raw) == "null" {
		return speedFieldInput{}, nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return speedFieldInput{}, errInvalidOutspeedBody
	}

	input := speedFieldInput{}
	for key, value := range fields {
		if !speedFieldKeys[key] {
			return speedFieldInput{}, errInvalidOutspeedBody
		}
		var b bool
		if err := json.Unmarshal(value, &b); err != nil {
			return speedFieldInput{}, errInvalidOutspeedBody
		}
		switch key {
		case "trickRoom":
			input.trickRoom = b
		case "attackerTailwind":
			input.attackerTailwind = b
		case "defenderTailwind":
			input.defenderTailwind = b
		}
	}
	return input, nil
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
	if !validNatureID(*wire.NatureID) {
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

// toDefenderInput validates a DefenderCandidate: the shared Individual fields (via
// toIndividualInput) plus the required, non-empty moveId (ADR-0704 §1・受け入れ条件1).
func toDefenderInput(wire defenderCandidateWire) (defenderInput, error) {
	individual, err := toIndividualInput(wire.individualWire)
	if err != nil {
		return defenderInput{}, err
	}
	moveID, ok := decodeRequiredString(wire.MoveID)
	if !ok {
		return defenderInput{}, errInvalidOutspeedBody
	}
	if !validMoveID(moveID) {
		return defenderInput{}, errInvalidOutspeedBody
	}
	return defenderInput{individualInput: individual, moveID: moveID}, nil
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

// toReverseClientField builds the field sent to calc-svc for the reverse calc (this candidate →
// self): attackerScreens and defenderScreens swap because the attacking side swaps, but weather/
// terrain don't (they're whole-field state, not tied to either side. ADR-0704 §4). nil stays
// nil (a request without field sends no field in either direction).
func toReverseClientField(wire *api.FieldState) *client.FieldState {
	field := toClientField(wire)
	if field == nil {
		return nil
	}
	field.AttackerScreens, field.DefenderScreens = field.DefenderScreens, field.AttackerScreens
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
// nature list, for either side. who is "attacker" or "defenders[<index>]" (ADR-0703 §3): index
// never appears for an attacker-side failure, and a candidate failure always names its index.
func writeUnknownNature(c *echo.Context, who string) error {
	return c.JSON(http.StatusUnprocessableEntity, api.Error{
		Code:    api.UnknownNature,
		Message: fmt.Sprintf("natureId is not in the nature list (%s)", who),
	})
}

// writeSpeciesError maps a Pokedex.Species failure to the ADR-0701 §6 table: ErrNotFound is
// unknown_species (422); everything else (unreachable/timeout/5xx/invalid response, or a
// pokedex-svc 400 the contract doesn't expect at this point) folds into upstream_unavailable,
// same as writeUpstreamError. who names the failing side (ADR-0703 §3).
func writeSpeciesError(c *echo.Context, err error, who string) error {
	if errors.Is(err, client.ErrNotFound) {
		return c.JSON(http.StatusUnprocessableEntity, api.Error{
			Code:    api.UnknownSpecies,
			Message: fmt.Sprintf("speciesKey is not in the pokedex master (%s)", who),
		})
	}
	return writeUpstreamError(c, err)
}

// writeMoveError maps a Pokedex.Move failure to the ADR-0704 §6 table: ErrNotFound is
// unknown_move (422); everything else folds into upstream_unavailable, same as writeSpeciesError.
// who names the failing side (attacker's own move, or defenders[<index>]'s move; ADR-0703 §3).
func writeMoveError(c *echo.Context, err error, who string) error {
	if errors.Is(err, client.ErrNotFound) {
		return c.JSON(http.StatusUnprocessableEntity, api.Error{
			Code:    api.UnknownMove,
			Message: fmt.Sprintf("moveId is not in the pokedex master (%s)", who),
		})
	}
	return writeUpstreamError(c, err)
}

// writeCalcError maps a Calc.Damage failure to the ADR-0701 §6 table: calc-svc's own 400
// (unknown move/item/ability, SP over budget, ...) folds into invalid_request, because judge
// cannot distinguish which one it was without reading calc-svc's body (ADR-0700 §3). Everything
// else folds into upstream_unavailable. who names the candidate the calc call was for
// (ADR-0703 §3; calc is always for one of the defenders, never attacker alone).
func writeCalcError(c *echo.Context, err error, who string) error {
	if errors.Is(err, client.ErrInvalidRequest) {
		return c.JSON(http.StatusBadRequest, api.Error{
			Code:    api.InvalidRequest,
			Message: fmt.Sprintf("calc-svc did not accept the calculation request (%s)", who),
		})
	}
	return writeUpstreamError(c, err)
}

// writeUpstreamError answers 503 upstream_unavailable (ADR-0701 §6): pokedex-svc/calc-svc
// unreachable, timed out, 5xx, or a response that doesn't match the contract. The message never
// includes upstream detail (ADR-0700 §3), but err is still logged so operators have a cause to
// look at (ADR-0700 §3 "ログには残す"; same as internalError).
func writeUpstreamError(c *echo.Context, err error) error {
	slog.Warn("judge upstream unavailable", "path", c.Path(), "error", err)
	return c.JSON(http.StatusServiceUnavailable, api.Error{
		Code:    api.UpstreamUnavailable,
		Message: "upstream is unavailable",
	})
}
