package httpapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/speed/internal/adapters/httpguard"
	"example.com/pokecalc/services/speed/internal/api"
	"example.com/pokecalc/services/speed/internal/domain"
)

// I-speed-4 / ADR-0610 §2 のハンドラ。判定順は /position と同じ:
// body(400。サイズ超過は 413)→ read model なし(503)→ 未知の pokemonId(422)→ 200。
// ヘッダーの検査は requireRequestContext ミドルウェア(New で登録)が body より先に行う。

// maxMatchupAssumptions は相手の想定の最大数(openapi.yaml の MatchupOpponent.assumptions の maxItems)。
const maxMatchupAssumptions = 8

var errInvalidMatchupBody = errors.New("request body is not a valid matchup request")

// GetSpeedMatchup implements POST /api/speed/v1/matchup (ADR-0610 §2).
func (h handler) GetSpeedMatchup(c *echo.Context, _ api.GetSpeedMatchupParams) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxPositionBodyBytes)
	body, err := decodeJSONBody[api.MatchupRequest](c.Request())
	if err != nil {
		return bodyError(c, err)
	}
	req, err := toMatchupRequest(body)
	if err != nil {
		return bodyError(c, err)
	}

	roster, ok, err := h.loadRoster(c)
	if !ok {
		return err
	}
	if httpguard.Expired(c.Request().Context()) {
		return overloaded(c)
	}
	result, err := domain.Matchup(roster, req)
	if err != nil {
		return coreError(c, err)
	}
	return c.JSON(http.StatusOK, toMatchupResponse(result, req))
}

// GetSpeedReach implements POST /api/speed/v1/reach (ADR-0610 §2). The body is a PositionRequest
// (validated exactly as /position does), and tableTailwind is the opponent-side tailwind.
func (h handler) GetSpeedReach(c *echo.Context, _ api.GetSpeedReachParams) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxPositionBodyBytes)
	body, err := decodeJSONBody[api.PositionRequest](c.Request())
	if err != nil {
		return bodyError(c, err)
	}
	req, err := toPositionRequest(body)
	if err != nil {
		return bodyError(c, err)
	}

	roster, ok, err := h.loadRoster(c)
	if !ok {
		return err
	}
	if httpguard.Expired(c.Request().Context()) {
		return overloaded(c)
	}
	// 自分の素早さ(と表示用のポケモン)は /position と同じ計算で求める。
	position, err := domain.Position(roster, req)
	if err != nil {
		return coreError(c, err)
	}
	lines, err := domain.Reach(roster, position.Speed, req.TableTailwind)
	if err != nil {
		return internalError(c, err)
	}
	return c.JSON(http.StatusOK, toReachResponse(position, lines))
}

// loadRoster は read model を取り出す。ok が false なら、応答(503 / 500)は書き込み済みで err を返すだけでよい。
func (h handler) loadRoster(c *echo.Context) (roster domain.Roster, ok bool, err error) {
	if h.deps.Pokemon == nil {
		return domain.Roster{}, false, c.JSON(http.StatusServiceUnavailable, api.Error{
			Code:    api.MasterUnavailable,
			Message: "the pokemon read model is not configured",
		})
	}
	roster, loadErr := h.deps.Pokemon.Roster()
	if loadErr != nil {
		return domain.Roster{}, false, internalError(c, loadErr)
	}
	return roster, true, nil
}

// bodyError は body の読み取り・検証の失敗を 413 / 400 にする。
func bodyError(c *echo.Context, err error) error {
	if errors.Is(err, errPositionBodyTooLarge) {
		return c.JSON(http.StatusRequestEntityTooLarge, api.Error{Code: api.RequestTooLarge, Message: err.Error()})
	}
	return c.JSON(http.StatusBadRequest, api.Error{Code: api.InvalidRequest, Message: err.Error()})
}

// coreError はコアのエラーを 422(未知のポケモン)か 500 にする。入力はここまでに自前で検証済みなので、
// 他のエラーは想定外(固定文言の 500。ADR-0602 §4)。
func coreError(c *echo.Context, err error) error {
	if errors.Is(err, domain.ErrUnknownPokemon) {
		return c.JSON(http.StatusUnprocessableEntity, api.Error{
			Code:    api.UnknownPokemon,
			Message: "pokemonId is not in the read model",
		})
	}
	return internalError(c, err)
}

// toMatchupRequest は生成型を検証してコアの入力に詰め替える。read model を使わずに検証できるものはここで済ませる
// (read model 未設定でも 503 ではなく 400 を返すため)。範囲の検査はコアの Speed に任せる(上限・下限を複製しない)。
func toMatchupRequest(body api.MatchupRequest) (domain.MatchupRequest, error) {
	if !pokemonIDPattern.MatchString(body.Self.PokemonId) || !pokemonIDPattern.MatchString(body.Opponent.PokemonId) {
		return domain.MatchupRequest{}, errInvalidMatchupBody
	}
	self := domain.Condition{
		Rank:      optionalInt(body.Self.Rank),
		Scarf:     optionalBool(body.Self.Scarf),
		Tailwind:  optionalBool(body.Self.Tailwind),
		Paralysis: optionalBool(body.Self.Paralysis),
	}
	if !validSpeedFields(0, domain.NatureNeutral, self.Rank) {
		return domain.MatchupRequest{}, errInvalidMatchupBody
	}
	var current *domain.Investment
	if body.Self.Current != nil {
		nature := domain.NatureEffect(body.Self.Current.Nature)
		if !validSpeedFields(body.Self.Current.Sp, nature, 0) {
			return domain.MatchupRequest{}, errInvalidMatchupBody
		}
		current = &domain.Investment{SP: body.Self.Current.Sp, Nature: nature}
	}
	assumptions, err := toAssumptions(body.Opponent.Assumptions)
	if err != nil {
		return domain.MatchupRequest{}, err
	}
	return domain.MatchupRequest{
		SelfPokemonID:     body.Self.PokemonId,
		Self:              self,
		SelfCurrent:       current,
		OpponentPokemonID: body.Opponent.PokemonId,
		Assumptions:       assumptions,
		OpponentTailwind:  optionalBool(body.Opponent.Tailwind),
		OpponentParalysis: optionalBool(body.Opponent.Paralysis),
		TrickRoom:         optionalBool(body.TrickRoom),
	}, nil
}

// toAssumptions は相手の想定を検証して詰め替える。省略(nil)は nil のまま(コアの既定)。
// 空配列・上限超え・preset と custom の両方 / どちらも無し・未知の preset・preset の重複は拒否する。
func toAssumptions(in *[]api.OpponentAssumption) ([]domain.OpponentAssumption, error) {
	if in == nil {
		return nil, nil
	}
	if len(*in) == 0 || len(*in) > maxMatchupAssumptions {
		return nil, errInvalidMatchupBody
	}
	seen := map[api.PresetId]bool{}
	out := make([]domain.OpponentAssumption, 0, len(*in))
	for _, a := range *in {
		if (a.Preset == nil) == (a.Custom == nil) {
			return nil, errInvalidMatchupBody
		}
		if a.Preset != nil {
			if !a.Preset.Valid() || seen[*a.Preset] {
				return nil, errInvalidMatchupBody
			}
			seen[*a.Preset] = true
			out = append(out, domain.OpponentAssumption{Preset: domain.PresetID(*a.Preset)})
			continue
		}
		custom := domain.OpponentCustom{
			SP:     a.Custom.Sp,
			Nature: domain.NatureEffect(a.Custom.Nature),
			Rank:   optionalInt(a.Custom.Rank),
			Scarf:  optionalBool(a.Custom.Scarf),
		}
		if !validSpeedFields(custom.SP, custom.Nature, custom.Rank) {
			return nil, errInvalidMatchupBody
		}
		out = append(out, domain.OpponentAssumption{Custom: &custom})
	}
	return out, nil
}

// validSpeedFields は SP・性格・ランクの範囲を、コアの Speed に有効な種族値を渡して確かめる。
func validSpeedFields(sp int, nature domain.NatureEffect, rank int) bool {
	_, err := domain.Speed(domain.Input{BaseSpeed: 1, SP: sp, Nature: nature, Rank: rank})
	return err == nil
}

func optionalInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func toSpeedPokemon(p domain.Pokemon) api.SpeedPokemon {
	types := make([]string, len(p.Types))
	copy(types, p.Types)
	return api.SpeedPokemon{PokemonId: p.PokemonID, NameJa: p.NameJa, Types: types, BaseSpeed: p.BaseSpeed}
}

func toSpRange(r *domain.SPRange) *api.SpRange {
	if r == nil {
		return nil
	}
	return &api.SpRange{Min: r.Min, Max: r.Max}
}

func toBaseSpeedRange(r *domain.BaseRange) *api.BaseSpeedRange {
	if r == nil {
		return nil
	}
	return &api.BaseSpeedRange{Min: r.Min, Max: r.Max}
}

func toMatchupResponse(result domain.MatchupResult, req domain.MatchupRequest) api.MatchupResponse {
	lines := make([]api.MatchupLine, len(result.Lines))
	for i, l := range result.Lines {
		natures := make([]api.NatureLine, len(l.Lines))
		for j, n := range l.Lines {
			natures[j] = api.NatureLine{Nature: api.NatureId(n.Nature), MovesFirst: toSpRange(n.MovesFirst), Tie: toSpRange(n.Tie)}
		}
		assumption := api.OpponentAssumption{}
		if l.Assumption.Custom != nil {
			c := l.Assumption.Custom
			custom := api.OpponentCustom{Sp: c.SP, Nature: api.NatureId(c.Nature)}
			if c.Rank != 0 {
				custom.Rank = &c.Rank
			}
			if c.Scarf {
				custom.Scarf = &c.Scarf
			}
			assumption.Custom = &custom
		} else {
			preset := api.PresetId(l.Assumption.Preset)
			assumption.Preset = &preset
		}
		line := api.MatchupLine{Assumption: assumption, OpponentSpeed: l.OpponentSpeed, Natures: natures}
		if l.Current != nil {
			order := api.TurnOrder(*l.Current)
			line.Current = &order
		}
		lines[i] = line
	}
	response := api.MatchupResponse{
		Self:      toSpeedPokemon(result.Self),
		Opponent:  toSpeedPokemon(result.Opponent),
		Lines:     lines,
		TrickRoom: req.TrickRoom,
	}
	// selfSpeed は self.current を渡したときだけ付ける(値が 0 かどうかでは判定しない)。
	if req.SelfCurrent != nil {
		speedValue := result.SelfSpeed
		response.SelfSpeed = &speedValue
	}
	return response
}

func toReachResponse(position domain.PositionResult, lines []domain.ReachLine) api.ReachResponse {
	out := make([]api.ReachLine, len(lines))
	for i, l := range lines {
		line := api.ReachLine{
			Preset:      api.PresetId(l.Preset),
			Slower:      toBaseSpeedRange(l.Slower),
			Tie:         toBaseSpeedRange(l.Tie),
			Faster:      toBaseSpeedRange(l.Faster),
			SlowerCount: l.SlowerCount,
			TieCount:    l.TieCount,
			FasterCount: l.FasterCount,
		}
		if l.FastestSlower != nil {
			p := toSpeedPokemon(*l.FastestSlower)
			line.FastestSlower = &p
		}
		if l.SlowestFaster != nil {
			p := toSpeedPokemon(*l.SlowestFaster)
			line.SlowestFaster = &p
		}
		out[i] = line
	}
	response := api.ReachResponse{Speed: position.Speed, Lines: out}
	if position.Pokemon != nil {
		p := toSpeedPokemon(*position.Pokemon)
		response.Pokemon = &p
	}
	return response
}
