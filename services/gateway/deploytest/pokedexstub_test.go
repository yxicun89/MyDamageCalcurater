package deploytest_test

// スモークスクリプトのテスト用の偽 pokedex-svc(ADR-0206 §3・AC-P6〜P8)。
//
// k3d の pokedex-svc(実データ)は使えないので、calc-svc が読むのと同じ例のマスタ
// (services/calc/testdata/master.example.json。架空データ。ADR-0002)から公開 API の応答を作る。
// 応答の型は生成型(services/internal/api)なので、契約と同じ形・同じ JSON のキー(無補正の性格の
// plus / minus が omitempty で落ちることを含む)になる。スモークが ID をここから引ければ、
// calc-svc(同じマスタ)の計算もそのまま通る。

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"sync"
	"testing"

	"example.com/pokecalc/services/gateway/deploytest"
	"example.com/pokecalc/services/internal/api"
)

// pokedexMode は偽 pokedex-svc の状態(スモークが入手元をどう見分けるか。ADR-0206 §4)。
type pokedexMode int

const (
	pokedexUnset             pokedexMode = iota // GATEWAY_POKEDEX_URL 未設定(→ 503 upstream_unavailable)。`make dev` 相当
	pokedexServesMaster                         // 投入済みの pokedex-svc。例のマスタから作った natures / species / moves を返す
	pokedexEmptyDB                              // DB 未投入(全操作が 503 master_unavailable)。`make up` 直後の k3d 相当
	pokedexNoNeutralNature                      // 200 だが無補正の性格が1つも無い(逆算の探索が届かない)
	pokedexNoDamagingMove                       // 200 だが威力のある物理技が1つも無い
	pokedexNoSpecies                            // 200 だが種族が空
	pokedexFirstMoveUnusable                    // 200。候補の先頭が計算するとダメージ 0 になる技(2番目で成立する)
	pokedexOnlyUnusableMoves                    // 200。候補がすべて計算するとダメージ 0 になる技
	pokedexServerError                          // 500(未知の最終状態)
	pokedexNotFound                             // 404(ルートが無い)
)

// unusableMove は「一覧では威力のある物理技に見えるが、実際に計算するとダメージが 0 になる技」。
// 本物の pokedex-svc では相性で無効になる組み合わせ(ゴースト技 → ノーマル など)がこれに当たるが、
// 例のマスタの相性表と種族にはその組み合わせが無いので、変化技(testglare。威力 0)を物理・威力ありと
// 偽って返すことで代用する。スモークが候補を順に試すこと(AC-P6)と、使える技が1つも無ければ失敗すること
// (AC-P8)を確かめるために使う。
func unusableMove(export api.MasterExport) api.Move {
	for _, m := range export.Moves {
		if m.Category == api.Status {
			priority := m.Priority
			return api.Move{Id: m.Id, NameJa: m.NameJa, Type: m.Type, Category: api.Physical, Power: 40, Priority: &priority}
		}
	}
	panic("例のマスタに変化技が無い(偽 pokedex-svc の前提が崩れている)")
}

// pokedexRecorder は偽 pokedex-svc が受けたパスを記録する層。スモークが「pokedex から ID を引いた」と
// 名乗るだけでなく、実際に公開 API を叩いたことを確かめるために使う(空振り防止)。
type pokedexRecorder struct {
	next http.Handler

	mu    sync.Mutex
	paths []string
}

func (r *pokedexRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	r.paths = append(r.paths, req.URL.Path)
	r.mu.Unlock()
	r.next.ServeHTTP(w, req)
}

// Paths は受けたパスを受けた順に返す。
func (r *pokedexRecorder) Paths() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.paths...)
}

// exampleMaster は calc-svc の例のマスタ(単一の正)を読む。
func exampleMaster(t *testing.T) api.MasterExport {
	t.Helper()
	raw, err := os.ReadFile(deploytest.RepoPath(t, "services/calc/testdata/master.example.json"))
	if err != nil {
		t.Fatalf("例のマスタを読めない: %v", err)
	}
	var export api.MasterExport
	if err := json.Unmarshal(raw, &export); err != nil {
		t.Fatalf("例のマスタを MasterExport として読めない: %v", err)
	}
	return export
}

// discoveredIDs は ADR-0206 §3 の選び方(無補正の性格の先頭・種族の先頭・威力のある物理技の先頭)を
// 例のマスタに当てはめた期待値。スモークは使った入手元と ID を出力に出すので、それと突き合わせる。
func discoveredIDs(t *testing.T) (natureID, speciesKey, moveID string) {
	t.Helper()
	export := exampleMaster(t)
	for _, n := range export.Natures {
		if n.Plus == nil && n.Minus == nil {
			natureID = n.Id
			break
		}
	}
	if len(export.Species) > 0 {
		speciesKey = string(export.Species[0].Key)
	}
	for _, m := range export.Moves {
		if m.Category == api.Physical && m.Power > 0 {
			moveID = m.Id
			break
		}
	}
	if natureID == "" || speciesKey == "" || moveID == "" {
		t.Fatalf("例のマスタから期待値を作れない(nature=%q species=%q move=%q)", natureID, speciesKey, moveID)
	}
	return natureID, speciesKey, moveID
}

// newPokedexStub は mode に応じた偽 pokedex-svc のハンドラを返す(gateway の上流に置く)。
// パスは公開 API(ADR-0105)と同じ /api/pokedex/natures・/api/pokedex/species・/api/pokedex/moves。
func newPokedexStub(t *testing.T, mode pokedexMode) http.Handler {
	t.Helper()
	export := exampleMaster(t)

	natures := make([]api.Nature, 0, len(export.Natures))
	for _, n := range export.Natures {
		if mode == pokedexNoNeutralNature && n.Plus == nil && n.Minus == nil {
			continue
		}
		natures = append(natures, api.Nature{Id: n.Id, NameJa: n.NameJa, Plus: n.Plus, Minus: n.Minus})
	}
	species := make([]api.SpeciesSummary, 0, len(export.Species))
	if mode != pokedexNoSpecies {
		for _, s := range export.Species {
			types := []api.PokeType{s.Type1}
			if s.Type2 != nil {
				types = append(types, *s.Type2)
			}
			species = append(species, api.SpeciesSummary{Key: s.Key, NameJa: s.NameJa, DexNo: s.DexNo, Form: s.Form, Types: types})
		}
	}
	moves := make([]api.Move, 0, len(export.Moves)+1)
	if mode == pokedexFirstMoveUnusable || mode == pokedexOnlyUnusableMoves {
		moves = append(moves, unusableMove(export))
	}
	if mode != pokedexOnlyUnusableMoves {
		for _, m := range export.Moves {
			if mode == pokedexNoDamagingMove && m.Power > 0 && m.Category == api.Physical {
				continue
			}
			priority := m.Priority
			moves = append(moves, api.Move{Id: m.Id, NameJa: m.NameJa, Type: m.Type, Category: m.Category, Power: m.Power, Priority: &priority})
		}
	}

	writeJSON := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch mode {
		case pokedexEmptyDB:
			writeJSON(w, http.StatusServiceUnavailable, api.Error{Code: api.MasterUnavailable, Message: "データが未投入"})
			return
		case pokedexServerError:
			writeJSON(w, http.StatusInternalServerError, api.Error{Code: api.Internal, Message: "内部エラー"})
			return
		case pokedexNotFound:
			writeJSON(w, http.StatusNotFound, api.Error{Code: api.NotFound, Message: "ルートが無い"})
			return
		}
		// limit は先頭から切り出すだけ(並びは ID 順という契約に合わせ、例のマスタの並びをそのまま使う)。
		limit := func(n int) int {
			raw := r.URL.Query().Get("limit")
			if raw == "" {
				return n
			}
			v, err := strconv.Atoi(raw)
			if err != nil || v < 0 || v > n {
				return n
			}
			return v
		}
		switch r.URL.Path {
		case "/api/pokedex/natures":
			writeJSON(w, http.StatusOK, natures)
		case "/api/pokedex/species":
			writeJSON(w, http.StatusOK, species[:limit(len(species))])
		case "/api/pokedex/moves":
			writeJSON(w, http.StatusOK, moves[:limit(len(moves))])
		default:
			writeJSON(w, http.StatusNotFound, api.Error{Code: api.NotFound, Message: "ルートが無い"})
		}
	})
}
