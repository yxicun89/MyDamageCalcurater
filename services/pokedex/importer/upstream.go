package importer

// 上流(npm の @smogon/calc・smogon/pokemon-showdown・PokeAPI/pokeapi)の最新版の検出結果の
// 読み込みと、固定版(config.json の sources)との比較(ADR-0104 §1・§4・§10)。
//
// ネットワークへの問い合わせは tools/importer/check-upstream.mjs(Node)が行い、結果を
// data/generated/upstream/latest.json に書く。ここではそのファイルを読んで比べるだけ
// (Go の importer は実行時にネットワークへ出ない。ADR-0101 §1)。

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// UpstreamLatest は data/generated/upstream/latest.json の内容。
type UpstreamLatest struct {
	SchemaVersion int               `json:"schemaVersion"`
	CheckedAt     string            `json:"checkedAt"` // RFC3339
	Sources       map[string]string `json:"sources"`
	Errors        map[string]string `json:"errors"`
}

// DecodeUpstreamLatest は raw を厳格にデコードする(未知のフィールド・schemaVersion の食い違い・
// checkedAt の形式・source/版の形式・sources と errors の重複・後続データを拒否)。
func DecodeUpstreamLatest(raw []byte) (UpstreamLatest, error) {
	var u UpstreamLatest
	if err := strictDecode(raw, &u); err != nil {
		return UpstreamLatest{}, err
	}
	if err := checkSchemaVersion(u.SchemaVersion); err != nil {
		return UpstreamLatest{}, err
	}
	if u.CheckedAt == "" {
		return UpstreamLatest{}, fmt.Errorf("%w: checkedAt が空", ErrInvalidInput)
	}
	if _, err := time.Parse(time.RFC3339, u.CheckedAt); err != nil {
		return UpstreamLatest{}, fmt.Errorf("%w: checkedAt の形式が不正(RFC3339 であること): %q", ErrInvalidInput, u.CheckedAt)
	}
	for _, source := range sortedKeysRaw(u.Sources) {
		if !sourcePattern.MatchString(source) {
			return UpstreamLatest{}, fmt.Errorf("%w: sources の source の形式が不正: %q", ErrInvalidInput, source)
		}
		v := u.Sources[source]
		if v == "" {
			return UpstreamLatest{}, fmt.Errorf("%w: sources.%s の版が空", ErrInvalidInput, source)
		}
		if strings.ContainsFunc(v, unicode.IsSpace) {
			return UpstreamLatest{}, fmt.Errorf("%w: sources.%s の版に空白を含む: %q", ErrInvalidInput, source, v)
		}
		if _, dup := u.Errors[source]; dup {
			return UpstreamLatest{}, fmt.Errorf("%w: source %q が sources と errors の両方にある", ErrInvalidInput, source)
		}
	}
	for _, source := range sortedKeysRaw(u.Errors) {
		if !sourcePattern.MatchString(source) {
			return UpstreamLatest{}, fmt.Errorf("%w: errors の source の形式が不正: %q", ErrInvalidInput, source)
		}
		if u.Errors[source] == "" {
			return UpstreamLatest{}, fmt.Errorf("%w: errors.%s の理由が空", ErrInvalidInput, source)
		}
	}
	return u, nil
}

// UpstreamState は固定版と上流の版の比較結果。
type UpstreamState string

const (
	UpstreamSame    UpstreamState = "same"
	UpstreamDiffers UpstreamState = "differs"
	UpstreamUnknown UpstreamState = "unknown"
)

// UpstreamStatus は1 source の比較結果(ADR-0104 §4・§10)。
type UpstreamStatus struct {
	Source string
	Pinned string
	Latest string
	State  UpstreamState
	Detail string
}

// CompareUpstream は pinned(config.json の sources)の source ごとに(昇順)、latest との
// 比較結果を返す。latest にだけある source は無視する。
//
//   - checkedAt が解釈できない、または now - checkedAt > maxAge なら全 source が unknown(stale)。
//   - errors にある source は unknown(理由は Errors の値)。
//   - 検出結果(sources・errors のどちらにも)無い source は unknown(not-checked)。
//   - それ以外は版が同じなら same、違えば differs。
func CompareUpstream(pinned map[string]string, latest UpstreamLatest, now time.Time, maxAge time.Duration) []UpstreamStatus {
	stale := true
	if checkedAt, err := time.Parse(time.RFC3339, latest.CheckedAt); err == nil {
		stale = now.Sub(checkedAt) > maxAge
	}
	sources := sortedKeysRaw(pinned)
	out := make([]UpstreamStatus, 0, len(sources))
	for _, source := range sources {
		st := UpstreamStatus{Source: source, Pinned: pinned[source]}
		switch {
		case stale:
			st.State = UpstreamUnknown
			st.Detail = "stale(前回の検出結果が古い)"
		case latest.Errors[source] != "":
			st.State = UpstreamUnknown
			st.Detail = latest.Errors[source]
		default:
			if v, ok := latest.Sources[source]; ok {
				st.Latest = v
				if v == st.Pinned {
					st.State = UpstreamSame
				} else {
					st.State = UpstreamDiffers
				}
			} else {
				st.State = UpstreamUnknown
				st.Detail = "not-checked"
			}
		}
		out = append(out, st)
	}
	return out
}

// FormatUpstream は決定的な表示を返す(kubectl logs で読む)。differs の行だけ "UPSTREAM" を
// 含める(ADR-0104 §4)。
func FormatUpstream(statuses []UpstreamStatus) string {
	var b strings.Builder
	for _, s := range statuses {
		switch s.State {
		case UpstreamDiffers:
			fmt.Fprintf(&b, "UPSTREAM %s: 固定版 %s → 上流 %s(data/importer/config.json を PR で更新すること)\n",
				s.Source, s.Pinned, s.Latest)
		case UpstreamUnknown:
			fmt.Fprintf(&b, "upstream %s: unknown(固定版 %s。%s)\n", s.Source, s.Pinned, s.Detail)
		default:
			fmt.Fprintf(&b, "upstream %s: same(%s)\n", s.Source, s.Pinned)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
