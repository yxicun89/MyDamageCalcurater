package official

import "strings"

// RobotsProduct は robots.txt の User-agent で自分を表す名前(fetcher.UserAgent の「/」の前。大文字小文字は区別しない)。
const RobotsProduct = "wishlist-price-checker"

// Robots は解析した robots.txt のうち、自分に効く規則(User-agent が * のグループと RobotsProduct のグループの両方)。
type Robots struct {
	rules []robotsRule
}

type robotsRule struct {
	allow bool
	path  string
}

// ParseRobots は robots.txt の本文を解析する(RFC 9309 を基に、既定案で決めた点は docs/phase4-spec.md 4-3 の「robots.txt」)。
//
//   - 行の # 以降はコメント。項目名(User-agent・Allow・Disallow)は大文字小文字を区別しない。値の前後の空白は除く。知らない項目は無視する
//   - 連続する User-agent 行が 1 つのグループの対象。その後の Allow・Disallow がそのグループの規則(次の User-agent 行で新しいグループ)
//   - User-agent が「*」のグループと、product(大文字小文字を区別しない完全一致)のグループの規則を**両方**使う(厳しいほうに寄せる)
//   - 空の Disallow は規則にしない(全部許す)。空の Allow も規則にしない
//   - 最初の User-agent より前の Allow・Disallow は無視する
func ParseRobots(body string, product string) Robots {
	body = strings.TrimPrefix(body, "\uFEFF")
	var rules []robotsRule
	inGroup, applies, lastWasAgent := false, false, false
	for _, line := range strings.Split(body, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		switch key {
		case "user-agent":
			if !lastWasAgent {
				applies = false
			}
			inGroup, lastWasAgent = true, true
			if val == "*" || strings.EqualFold(val, product) {
				applies = true
			}
		case "allow", "disallow":
			lastWasAgent = false
			if inGroup && applies && val != "" {
				rules = append(rules, robotsRule{allow: key == "allow", path: val})
			}
		default:
			// 知らない項目は無視する(グループの区切りにもしない)
		}
	}
	return Robots{rules: rules}
}

// AllowAll は全部許す Robots(robots.txt が 404・410 のとき)。
func AllowAll() Robots { return Robots{} }

// Allowed は path(パス + 「?クエリ」。空なら「/」)を取ってよいか。
//
//   - 規則のパスが path に前方一致するものを集め、最も長い(バイト数)規則に従う。同じ長さなら Allow を優先する。一致が無ければ許す
//   - 規則のパスの「*」は 0 文字以上の任意の文字列、末尾の「$」は path の終わりに一致する(RFC 9309)。長さは規則の文字列のバイト数で比べる
//   - パスの大文字小文字は区別する。パーセントエンコードの正規化はしない(既定案)
func (r Robots) Allowed(path string) bool {
	if path == "" {
		path = "/"
	}
	bestLen, allowed := -1, true
	for _, r := range r.rules {
		if !robotsMatch(r.path, path) {
			continue
		}
		if n := len(r.path); n > bestLen || (n == bestLen && r.allow) {
			bestLen, allowed = n, r.allow
		}
	}
	return allowed
}

// robotsMatch は pattern(* と末尾の $ を持てる)が path の前方に一致するか。
// 動的計画法(O(len(pattern)×len(path)))で、* が多くても指数時間にならない。
func robotsMatch(pattern, path string) bool {
	anchored := strings.HasSuffix(pattern, "$")
	if anchored {
		pattern = strings.TrimSuffix(pattern, "$")
	}
	// cur[j] = pattern の先頭 i 文字が path の先頭 j 文字に一致する
	cur := make([]bool, len(path)+1)
	cur[0] = true
	for i := 0; i < len(pattern); i++ {
		next := make([]bool, len(path)+1)
		if pattern[i] == '*' {
			seen := false
			for j := 0; j <= len(path); j++ {
				seen = seen || cur[j]
				next[j] = seen
			}
		} else {
			for j := 0; j < len(path); j++ {
				next[j+1] = cur[j] && path[j] == pattern[i]
			}
		}
		cur = next
	}
	for j, ok := range cur {
		if ok && (!anchored || j == len(path)) {
			return true
		}
	}
	return false
}
