package master

// MasterExport(pokedex-svc の内部 API GET /internal/pokedex/master の本文。ADR-0204)から Store を作る入口と、
// その入手元(Source)。
//
// 相性表・種族・技・持ち物・特性は共通マスタ(services/internal/master)の写像にそのまま委ねる(値の検証も
// そちら任せ)。共通マスタが検証しないもの(ID の重複・種族/技/持ち物/特性の集合をまたぐ参照・性格・
// トップレベルの enum の綴り)だけを、この calc-svc 側で確かめる。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
	sharedmaster "example.com/pokecalc/services/internal/master"
)

// ロード・取得の失敗。呼び出し側は errors.Is で判別する。
var (
	// ErrInvalidMaster はマスタ一式が不正(壊れた JSON・未知のフィールド・後続のデータ・schemaVersion が 1 でない・
	// dataVersion が空・ID の重複・参照先が無い・性格の不正、および共通マスタの写像のエラー)。
	// 共通マスタ(services/internal/master)の ErrInvalidRow / ErrInvalidEffect、engine.ErrInvalidTypeChart も
	// 同時に errors.Is で判別できるように包む。
	ErrInvalidMaster = errors.New("マスタ一式が不正")
	// ErrMasterUnavailable はマスタ一式を取得できない(接続できない・タイムアウト・200 以外の応答)。
	ErrMasterUnavailable = errors.New("マスタ一式を取得できない")
)

// internalMasterPath は pokedex-svc の内部 API のパス(api/openapi.yaml の getMasterExport)。
const internalMasterPath = "/internal/pokedex/master"

// masterExportRequiredFields はトップレベルの必須フィールド(ADR-0204 §1)。構造体デコードは
// 欠落・null を素通りさせる(ゼロ値・nil スライスになるだけ)ため、別に確かめる。
var masterExportRequiredFields = []string{
	"schemaVersion", "dataVersion", "types", "typeChart", "species", "moves", "items", "abilities", "natures",
}

// FromExport はマスタ一式を検証し、engine の型にしてメモリに持つ Store を作る(ADR-0204 §2)。
// 相性表・種族・技・持ち物・特性は共通マスタ(services/internal/master)の写像で engine の型にする。
// 性格は calc-svc 側で検証する(ID が空・重複、plus/minus が StatKey でない・HP を指す)。
// 不正はすべて ErrInvalidMaster で包む(部分的な Store は返さない)。
func FromExport(export api.MasterExport) (*MemoryStore, error) {
	if export.SchemaVersion != api.MasterExportSchemaVersionN1 {
		return nil, fmt.Errorf("%w: schemaVersion は 1 でなければならない: %d", ErrInvalidMaster, export.SchemaVersion)
	}
	if export.DataVersion == "" {
		return nil, fmt.Errorf("%w: dataVersion が空", ErrInvalidMaster)
	}

	chart, err := buildTypeChart(export.Types, export.TypeChart)
	if err != nil {
		return nil, err
	}
	abilities, err := buildAbilities(export.Abilities, chart)
	if err != nil {
		return nil, err
	}
	items, err := buildItems(export.Items, chart)
	if err != nil {
		return nil, err
	}
	species, err := buildSpecies(export.Species, abilities, items, chart)
	if err != nil {
		return nil, err
	}
	moves, err := buildMoves(export.Moves, chart)
	if err != nil {
		return nil, err
	}
	natures, err := buildNatures(export.Natures)
	if err != nil {
		return nil, err
	}

	return &MemoryStore{
		species: species, moves: moves, items: items, abilities: abilities,
		natures: natures, chart: chart, dataVersion: export.DataVersion,
	}, nil
}

// buildTypeChart は types / typeChart の行を共通マスタの写像で engine.TypeChart にする。
// タイプの綴りが契約の列挙(api.PokeType)に無ければ、共通マスタの形式検査(正規表現)をすり抜けてしまうため、
// ここで確かめる。
func buildTypeChart(types []api.MasterType, chartRows []api.MasterTypeChartEntry) (engine.TypeChart, error) {
	rows := make([]sharedmaster.TypeRow, 0, len(types))
	for _, t := range types {
		if !t.Id.Valid() {
			return engine.TypeChart{}, fmt.Errorf("%w: 未知のタイプ %q", ErrInvalidMaster, t.Id)
		}
		rows = append(rows, sharedmaster.TypeRow{ID: string(t.Id), SortOrder: t.SortOrder, NameJa: t.NameJa})
	}
	chartRowsOut := make([]sharedmaster.TypeChartRow, 0, len(chartRows))
	for _, c := range chartRows {
		chartRowsOut = append(chartRowsOut, sharedmaster.TypeChartRow{
			AttackType: string(c.AttackType), DefenseType: string(c.DefenseType), Code: int(c.Code),
		})
	}
	chart, err := sharedmaster.TypeChart(rows, chartRowsOut)
	if err != nil {
		return engine.TypeChart{}, fmt.Errorf("%w: %w", ErrInvalidMaster, err)
	}
	return chart, nil
}

// buildAbilities は abilities の行を共通マスタの写像で engine.Ability にする。ID の重複は calc-svc 側で見る
// (共通マスタは1行ずつしか見ないため)。
func buildAbilities(list []api.MasterAbility, chart engine.TypeChart) (map[string]engine.Ability, error) {
	out := make(map[string]engine.Ability, len(list))
	for _, a := range list {
		if _, dup := out[a.Id]; dup {
			return nil, fmt.Errorf("%w: 特性IDが重複している: %q", ErrInvalidMaster, a.Id)
		}
		effect, err := effectBytes(a.Effect)
		if err != nil {
			return nil, fmt.Errorf("%w: 特性 %q の効果を読めない: %v", ErrInvalidMaster, a.Id, err)
		}
		ab, err := sharedmaster.Ability(sharedmaster.AbilityRow{ID: a.Id, NameJa: a.NameJa, Effect: effect}, chart)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidMaster, err)
		}
		out[a.Id] = ab
	}
	return out, nil
}

// buildItems は items の行を共通マスタの写像で engine.Item にする。ID の重複は calc-svc 側で見る。
func buildItems(list []api.MasterItem, chart engine.TypeChart) (map[string]engine.Item, error) {
	out := make(map[string]engine.Item, len(list))
	for _, it := range list {
		if _, dup := out[it.Id]; dup {
			return nil, fmt.Errorf("%w: 持ち物IDが重複している: %q", ErrInvalidMaster, it.Id)
		}
		effect, err := effectBytes(it.Effect)
		if err != nil {
			return nil, fmt.Errorf("%w: 持ち物 %q の効果を読めない: %v", ErrInvalidMaster, it.Id, err)
		}
		item, err := sharedmaster.Item(sharedmaster.ItemRow{ID: it.Id, NameJa: it.NameJa, Effect: effect}, chart)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidMaster, err)
		}
		out[it.Id] = item
	}
	return out, nil
}

// effectBytes は MasterEffect(map[string]interface{}。DecodeExport が UseNumber で読んだ数値は json.Number)を
// JSON にして、共通マスタの DecodeItemEffect / DecodeAbilityEffect にそのまま渡せる形にする。
// json.Number は文字面のまま JSON へ書き戻るので、5324.0 のような小数の見た目は保たれる(丸めない)。
func effectBytes(e *api.MasterEffect) ([]byte, error) {
	if e == nil {
		return nil, nil
	}
	return json.Marshal(*e)
}

// buildSpecies は species(+ 特性の行)を共通マスタの写像で engine.Species にする。種族キーの重複、
// 特性一覧・種族一覧・持ち物一覧をまたぐ参照(種族の特性・メガの元種族/メガストーン)は共通マスタが
// 見ないので、ここで確かめる。
func buildSpecies(
	list []api.MasterSpecies, abilities map[string]engine.Ability, items map[string]engine.Item, chart engine.TypeChart,
) (map[string]engine.Species, error) {
	keys := make(map[string]bool, len(list))
	for _, s := range list {
		if keys[string(s.Key)] {
			return nil, fmt.Errorf("%w: 種族キーが重複している: %q", ErrInvalidMaster, s.Key)
		}
		keys[string(s.Key)] = true
	}

	out := make(map[string]engine.Species, len(list))
	for _, s := range list {
		abilityRows := make([]sharedmaster.SpeciesAbilityRow, 0, len(s.Abilities))
		for _, a := range s.Abilities {
			if _, ok := abilities[a.AbilityId]; !ok {
				return nil, fmt.Errorf("%w: 種族 %q の特性 %q が特性一覧に無い", ErrInvalidMaster, s.Key, a.AbilityId)
			}
			abilityRows = append(abilityRows, sharedmaster.SpeciesAbilityRow{Slot: a.Slot, AbilityID: a.AbilityId})
		}
		if s.IsMega {
			if s.BaseSpeciesKey != nil && !keys[*s.BaseSpeciesKey] {
				return nil, fmt.Errorf("%w: 種族 %q の baseSpeciesKey %q が種族一覧に無い", ErrInvalidMaster, s.Key, *s.BaseSpeciesKey)
			}
			if s.RequiredItemId != nil {
				if _, ok := items[*s.RequiredItemId]; !ok {
					return nil, fmt.Errorf("%w: 種族 %q の requiredItemId %q が持ち物一覧に無い", ErrInvalidMaster, s.Key, *s.RequiredItemId)
				}
			}
		}

		row := sharedmaster.SpeciesRow{
			Key: string(s.Key), DexNo: s.DexNo, Form: s.Form, ShowdownID: s.ShowdownId, NameJa: s.NameJa,
			Type1:  string(s.Type1),
			BaseHP: s.BaseStats.Hp, BaseAtk: s.BaseStats.Atk, BaseDef: s.BaseStats.Def,
			BaseSpA: s.BaseStats.Spa, BaseSpD: s.BaseStats.Spd, BaseSpe: s.BaseStats.Spe,
			IsMega: s.IsMega,
		}
		if s.Type2 != nil {
			row.Type2 = string(*s.Type2)
		}
		if s.BaseSpeciesKey != nil {
			row.BaseSpeciesKey = *s.BaseSpeciesKey
		}
		if s.RequiredItemId != nil {
			row.RequiredItemID = *s.RequiredItemId
		}

		sp, err := sharedmaster.Species(row, abilityRows, chart)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidMaster, err)
		}
		out[string(s.Key)] = sp
	}
	return out, nil
}

// buildMoves は moves の行を共通マスタの写像で engine.Move にする。ID の重複は calc-svc 側で見る。
func buildMoves(list []api.MasterMove, chart engine.TypeChart) (map[string]engine.Move, error) {
	out := make(map[string]engine.Move, len(list))
	for _, m := range list {
		if _, dup := out[m.Id]; dup {
			return nil, fmt.Errorf("%w: 技IDが重複している: %q", ErrInvalidMaster, m.Id)
		}
		effect, err := effectBytes(m.Effect)
		if err != nil {
			return nil, fmt.Errorf("%w: 技 %q の効果を読めない: %v", ErrInvalidMaster, m.Id, err)
		}
		row := sharedmaster.MoveRow{
			ID: m.Id, NameJa: m.NameJa, Type: string(m.Type), Category: string(m.Category),
			Power: m.Power, Priority: m.Priority, Effect: effect,
		}
		mv, err := sharedmaster.Move(row, chart)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidMaster, err)
		}
		out[m.Id] = mv
	}
	return out, nil
}

// buildNatures は natures を検証する(ADR-0204 §2。データレーンにまだ natures テーブルが無いため
// calc-svc 側で行う): ID が空・重複でない、plus/minus が StatKey で HP を指さない。
func buildNatures(list []api.MasterNature) (map[string]engine.Nature, error) {
	out := make(map[string]engine.Nature, len(list))
	for _, n := range list {
		if n.Id == "" {
			return nil, fmt.Errorf("%w: 性格IDが空", ErrInvalidMaster)
		}
		if _, dup := out[n.Id]; dup {
			return nil, fmt.Errorf("%w: 性格IDが重複している: %q", ErrInvalidMaster, n.Id)
		}
		plus, err := natureStat(n.Plus)
		if err != nil {
			return nil, err
		}
		minus, err := natureStat(n.Minus)
		if err != nil {
			return nil, err
		}
		if plus == engine.StatHP || minus == engine.StatHP {
			return nil, fmt.Errorf("%w: 性格 %q の補正は HP を指せない", ErrInvalidMaster, n.Id)
		}
		out[n.Id] = engine.Nature{Plus: plus, Minus: minus}
	}
	return out, nil
}

// natureStat は性格の plus/minus(nil は無補正)を検証する。
func natureStat(s *api.StatKey) (engine.StatKey, error) {
	if s == nil {
		return "", nil
	}
	if !s.Valid() {
		return "", fmt.Errorf("%w: 性格補正に未知のステータスキー %q", ErrInvalidMaster, *s)
	}
	return engine.StatKey(*s), nil
}

// maxMasterExportBytes はマスタ一式の本文の上限(バイト)。例のファイルは数 KB、実マスタ(全種族・技・
// 持ち物・特性)でも数 MB を超えない見込みだが、上流の異常時に無制限に読み込んでメモリを使い切らないよう
// 余裕を持った上限を設ける(critic 指摘)。
const maxMasterExportBytes = 16 << 20 // 16MiB

// errBodyTooLarge は readAllLimited が上限超過を伝えるための内部エラー(呼び出し側が包み直す)。
var errBodyTooLarge = errors.New("本文が上限を超える")

// readAllLimited は r から最大 limit バイトを読む。それを超えるデータがあれば errBodyTooLarge、
// 読み込み自体が失敗すれば(接続断・タイムアウト等)そのエラーをそのまま返す(呼び出し側が種別を判断する)。
func readAllLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errBodyTooLarge
	}
	return data, nil
}

// DecodeExport はマスタ一式の JSON を厳格に読む(未知のフィールド・後続のデータ・必須のトップレベルの
// 欠落/null を拒否する。効果定義の数値は字面のまま保つ: 5324.0 を 5324 に丸めて通さない)。
// 本文が maxMasterExportBytes を超える場合も含め、不正は ErrInvalidMaster で包む。
// (HTTPSource.Fetch は本文を読む段階の失敗を別に扱うため、readAllLimited と decodeExportBytes を直接使う。)
func DecodeExport(r io.Reader) (api.MasterExport, error) {
	data, err := readAllLimited(r, maxMasterExportBytes)
	if err != nil {
		if errors.Is(err, errBodyTooLarge) {
			return api.MasterExport{}, fmt.Errorf("%w: 本文が上限 %d バイトを超える", ErrInvalidMaster, maxMasterExportBytes)
		}
		return api.MasterExport{}, fmt.Errorf("%w: %v", ErrInvalidMaster, err)
	}
	return decodeExportBytes(data)
}

// decodeExportBytes は読み込み済みの本文を厳格にデコードする(DecodeExport と HTTPSource.Fetch の共通部分)。
func decodeExportBytes(data []byte) (api.MasterExport, error) {
	// 構造体デコードは欠落・null のフィールドを素通りさせる(ゼロ値・nil のまま)ため、
	// トップレベルの必須フィールドの有無・null は別に確かめる。
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return api.MasterExport{}, fmt.Errorf("%w: %v", ErrInvalidMaster, err)
	}
	for _, key := range masterExportRequiredFields {
		raw, ok := top[key]
		if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return api.MasterExport{}, fmt.Errorf("%w: 必須のフィールド %q が無いか null", ErrInvalidMaster, key)
		}
	}
	// items / abilities の各要素は effect キーを持つこと(値が null は「補正なし」として許す)。
	// *api.MasterEffect は「欠落」と「null」の両方が nil ポインタになって区別が付かなくなるため、
	// 構造体デコードの前に JSON の段階でキーの有無だけを確かめる。その他のオブジェクト内の必須は
	// ここでは見ない(共通マスタ・engine 側の検証、および ID の形式検査に委ねる。ADR-0204 §2)。
	if err := requireEffectField(top["items"], "items"); err != nil {
		return api.MasterExport{}, err
	}
	if err := requireEffectField(top["abilities"], "abilities"); err != nil {
		return api.MasterExport{}, err
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	var export api.MasterExport
	if err := dec.Decode(&export); err != nil {
		return api.MasterExport{}, fmt.Errorf("%w: %v", ErrInvalidMaster, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return api.MasterExport{}, fmt.Errorf("%w: JSON の後ろに余計なデータがある", ErrInvalidMaster)
	}
	return export, nil
}

// requireEffectField は items / abilities(raw はその JSON 配列)の各要素に "effect" キーがあることを
// 確かめる(値は見ない。null は許す。無いこと自体を ErrInvalidMaster にする)。
func requireEffectField(raw json.RawMessage, label string) error {
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return fmt.Errorf("%w: %s を読めない: %v", ErrInvalidMaster, label, err)
	}
	for i, e := range entries {
		if _, ok := e["effect"]; !ok {
			return fmt.Errorf("%w: %s[%d] に effect が無い", ErrInvalidMaster, label, i)
		}
	}
	return nil
}

// Source はマスタ一式の入手元(ADR-0204 §2)。
type Source interface {
	// Fetch はマスタ一式を1度取得する。ctx が終わったら ctx のエラーを包んで返す。
	Fetch(ctx context.Context) (api.MasterExport, error)
}

// FileSource は JSON ファイル(MasterExport の形)から読む入手元(k3d の local overlay と make dev 用)。
type FileSource struct {
	// Path は MasterExport の JSON ファイルのパス。
	Path string
}

var _ Source = FileSource{}

// Fetch は Source を実装する。ファイルが無い・読めない場合はそのエラーを、形が不正なら ErrInvalidMaster を包んで返す。
func (f FileSource) Fetch(_ context.Context) (api.MasterExport, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return api.MasterExport{}, err
	}
	return DecodeExport(bytes.NewReader(data))
}

// HTTPSource は pokedex-svc の内部 API(GET {BaseURL}/internal/pokedex/master)から取得する入手元。
type HTTPSource struct {
	baseURL string
	client  *http.Client
}

var _ Source = (*HTTPSource)(nil)

// NewHTTPSource は pokedex-svc のベース URL(http / https の絶対 URL。末尾の / は有っても無くてもよい)と、
// 1回の取得のタイムアウト(正の値)から HTTPSource を作る。URL・タイムアウトが不正ならエラー。
func NewHTTPSource(baseURL string, timeout time.Duration) (*HTTPSource, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("マスタ取得のタイムアウトは正でなければならない: %v", timeout)
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("ベース URL を解析できない: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("ベース URL は http/https の絶対 URL であること: %q", baseURL)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("ベース URL にホストが無い: %q", baseURL)
	}
	if u.RawQuery != "" {
		return nil, fmt.Errorf("ベース URL にクエリを含められない: %q", baseURL)
	}
	return &HTTPSource{
		baseURL: strings.TrimSuffix(u.String(), "/"),
		client:  &http.Client{Timeout: timeout},
	}, nil
}

// Fetch は Source を実装する。接続できない・タイムアウト・200 以外は ErrMasterUnavailable、
// 本文が不正(壊れた JSON・未知のフィールド)なら ErrInvalidMaster を包んで返す。
func (h *HTTPSource) Fetch(ctx context.Context) (api.MasterExport, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.baseURL+internalMasterPath, nil)
	if err != nil {
		return api.MasterExport{}, fmt.Errorf("%w: %v", ErrMasterUnavailable, err)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return api.MasterExport{}, ctxErr
		}
		return api.MasterExport{}, fmt.Errorf("%w: %v", ErrMasterUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return api.MasterExport{}, fmt.Errorf("%w: 上流が %d を返した", ErrMasterUnavailable, resp.StatusCode)
	}
	// 本文を読む段階の失敗(接続が途中で切れる・タイムアウト)は、JSON として不正なのではなく取得できて
	// いないだけなので ErrInvalidMaster にしない。ctx がすでに終わっていればそれを、そうでなければ
	// ErrMasterUnavailable を返す(critic 指摘)。上限超過だけは内容の不正として ErrInvalidMaster にする。
	data, err := readAllLimited(resp.Body, maxMasterExportBytes)
	if err != nil {
		if errors.Is(err, errBodyTooLarge) {
			return api.MasterExport{}, fmt.Errorf("%w: マスタ一式が上限 %d バイトを超える", ErrInvalidMaster, maxMasterExportBytes)
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return api.MasterExport{}, ctxErr
		}
		return api.MasterExport{}, fmt.Errorf("%w: %v", ErrMasterUnavailable, err)
	}
	return decodeExportBytes(data)
}
