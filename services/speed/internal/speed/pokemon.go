package speed

// Pokemon は素早さ比較に必要な 1 体分のマスタ(ADR-0600 §4)。
// Types は表示用(英小文字の ID)で、計算には使わない。
type Pokemon struct {
	PokemonID string
	NameJa    string
	Types     []string
	BaseSpeed int
}

// Roster は 1 つのレギュレーションで使用可能なポケモンの集合(ADR-0600 §4)。
// 並び順は provider に任せ、表示・API の並べ替えは呼び出し側が行う。
type Roster struct {
	RegulationID string
	Pokemon      []Pokemon
}

// PokemonProvider は read model の差し替え境界。SP0〜SP3 は speed 内の架空データ、
// SP4 で pokedex の read model の adapter に差し替える(ADR-0600 §4)。
// 返す Roster は呼び出し側が変更してもよい複製であること。
type PokemonProvider interface {
	Roster() (Roster, error)
}
