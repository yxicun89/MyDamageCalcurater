package official

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
	return Robots{} // TODO(implementer)
}

// AllowAll は全部許す Robots(robots.txt が 404・410 のとき)。
func AllowAll() Robots { return Robots{} }

// Allowed は path(パス + 「?クエリ」。空なら「/」)を取ってよいか。
//
//   - 規則のパスが path に前方一致するものを集め、最も長い(バイト数)規則に従う。同じ長さなら Allow を優先する。一致が無ければ許す
//   - 規則のパスの「*」は 0 文字以上の任意の文字列、末尾の「$」は path の終わりに一致する(RFC 9309)。長さは規則の文字列のバイト数で比べる
//   - パスの大文字小文字は区別する。パーセントエンコードの正規化はしない(既定案)
func (r Robots) Allowed(path string) bool {
	return false // TODO(implementer)
}
