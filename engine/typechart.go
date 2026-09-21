package engine

import (
	"errors"
	"fmt"
)

// --- P1-13: タイプ相性表をデータとして受け取る(ADR-0013 §P1-13)-------------
//
// タイプの一覧と相性表は**マスタ(データ)**で、engine は表をコードに持たず入力として受け取る。
// 相性の引き方(複合タイプの掛け合わせ・無効の扱い・TypeNone)は**ルール**なのでここに残す。
// 倍率は「×2 した整数コード」のまま扱い、float でダメージを近似しない。

// タイプ相性表に関する失敗。呼び出し側は errors.Is で判別する。
var (
	// ErrInvalidTypeChart は表の定義そのものが不正(空・重複・未知のキー・不正なコード)。
	ErrInvalidTypeChart = errors.New("タイプ相性表の定義が不正")
	// ErrTypeChartMissing は計算の入力に表が無い(TypeChart がゼロ値)。
	ErrTypeChartMissing = errors.New("タイプ相性表が入力に無い")
	// ErrUnknownType は入力に現れたタイプ ID が表に無い(黙って等倍にしない)。
	ErrUnknownType = errors.New("タイプ相性表に無いタイプ")
)

// 倍率コード。倍率を2倍した整数で表す(ADR-0013 §決定1)。
const (
	// TypeCodeImmune は無効(×0)。
	TypeCodeImmune = 0
	// TypeCodeNotVeryEffective はいまひとつ(×0.5)。
	TypeCodeNotVeryEffective = 1
	// TypeCodeNeutral は等倍(×1.0)。表に無い組はこの値とみなす。
	TypeCodeNeutral = 2
	// TypeCodeSuperEffective は抜群(×2.0)。
	TypeCodeSuperEffective = 4
)

// TypeChartData はマスタから渡すタイプ相性表の素データ(検証前)。
type TypeChartData struct {
	// Types は表に載るタイプ ID の集合。空・重複・TypeNone は不正。
	Types []Type
	// Effectiveness は [攻撃タイプ][防御タイプ] = 倍率コード(0/1/2/4)。
	// 等倍(2)は省略してよい(無い組は等倍)。ただしキーは Types に含まれていること。
	Effectiveness map[Type]map[Type]int
}

// TypeChart は検証済みのタイプ相性表。不変の値で、ゼロ値は「表が未設定」を表す。
// 計算に渡す前に NewTypeChart で作る。
type TypeChart struct {
	data *typeChartData
}

// typeChartData は検証済みの表の中身。添字表で引く(ADR-0013 §P1-13.7)。
type typeChartData struct {
	types []Type
	index map[Type]int
	codes []int8 // codes[攻撃の添字 * len(types) + 防御の添字]
}

// Effectiveness はタイプ相性の結果。倍率は Num/Den の整数比で保持し、約分しない
// (Den = 2^防御タイプ数)。float でダメージを近似しないため(CLAUDE.md ドメイン規約)。
type Effectiveness struct {
	Num int
	Den int
}

// Multiplier は表示用の倍率(0 / 0.25 / 0.5 / 1 / 2 / 4)を返す。ダメージ計算には使わない。
func (e Effectiveness) Multiplier() float64 {
	if e.Den == 0 {
		return 0
	}
	return float64(e.Num) / float64(e.Den)
}

// IsImmune は無効(×0)かどうかを返す。
func (e Effectiveness) IsImmune() bool { return e.Num == 0 }

// IsSuperEffective は抜群(等倍より大きい)かどうかを返す。
func (e Effectiveness) IsSuperEffective() bool { return e.Num > e.Den }

// noTypeIndex は TypeNone(タイプなし)を表す添字。表の添字にはならない値。
const noTypeIndex = -1

// NewTypeChart はマスタ由来の素データを検証して表を作る。
// 失敗は ErrInvalidTypeChart で包む。作られた表は不変で、引数の map を後から
// 書き換えても影響を受けない。
func NewTypeChart(d TypeChartData) (TypeChart, error) {
	if len(d.Types) == 0 {
		return TypeChart{}, fmt.Errorf("%w: Types が空", ErrInvalidTypeChart)
	}
	data := &typeChartData{
		types: append([]Type(nil), d.Types...),
		index: make(map[Type]int, len(d.Types)),
	}
	for i, t := range data.types {
		if t == TypeNone {
			return TypeChart{}, fmt.Errorf("%w: Types に TypeNone は置けない", ErrInvalidTypeChart)
		}
		if _, dup := data.index[t]; dup {
			return TypeChart{}, fmt.Errorf("%w: Types が重複している: %q", ErrInvalidTypeChart, t)
		}
		data.index[t] = i
	}
	if err := data.fillCodes(d.Effectiveness); err != nil {
		return TypeChart{}, err
	}
	return TypeChart{data: data}, nil
}

// fillCodes は等倍で埋めた添字表に、素データの倍率コードを書き込む。
// キーが Types に無い、またはコードが 0/1/2/4 以外なら ErrInvalidTypeChart。
func (d *typeChartData) fillCodes(effectiveness map[Type]map[Type]int) error {
	n := len(d.types)
	d.codes = make([]int8, n*n)
	for i := range d.codes {
		d.codes[i] = TypeCodeNeutral
	}
	for atk, row := range effectiveness {
		ai, ok := d.index[atk]
		if !ok {
			return fmt.Errorf("%w: 攻撃側のキー %q が Types に無い", ErrInvalidTypeChart, atk)
		}
		for def, code := range row {
			di, ok := d.index[def]
			if !ok {
				return fmt.Errorf("%w: 防御側のキー %q が Types に無い", ErrInvalidTypeChart, def)
			}
			if !isTypeCode(code) {
				return fmt.Errorf("%w: %q → %q のコード %d は 0/1/2/4 のどれでもない", ErrInvalidTypeChart, atk, def, code)
			}
			d.codes[ai*n+di] = int8(code)
		}
	}
	return nil
}

// isTypeCode は倍率コードとして定義された値(0/1/2/4)かどうかを返す。
func isTypeCode(code int) bool {
	switch code {
	case TypeCodeImmune, TypeCodeNotVeryEffective, TypeCodeNeutral, TypeCodeSuperEffective:
		return true
	}
	return false
}

// IsZero は表が未設定(ゼロ値)かどうかを返す。
func (c TypeChart) IsZero() bool { return c.data == nil }

// Types は表に載るタイプ ID を定義順で返す。呼び出しごとに新しいスライスを返す。
func (c TypeChart) Types() []Type {
	if c.data == nil {
		return nil
	}
	return append([]Type(nil), c.data.types...)
}

// Has はタイプ ID が表にあるかどうかを返す。TypeNone は常に false。
func (c TypeChart) Has(t Type) bool {
	if c.data == nil {
		return false
	}
	_, ok := c.data.index[t]
	return ok
}

// indexOf はタイプの添字を返す。TypeNone は noTypeIndex、表に無い ID は ErrUnknownType。
// 呼び出し側が表のゼロ値を先に弾いていること。
func (c TypeChart) indexOf(t Type) (int, error) {
	if t == TypeNone {
		return noTypeIndex, nil
	}
	i, ok := c.data.index[t]
	if !ok {
		return 0, fmt.Errorf("%w: %q", ErrUnknownType, t)
	}
	return i, nil
}

// codeAt は添字で引いた倍率コードを返す。どちらかがタイプなしなら等倍。
func (c TypeChart) codeAt(atk, def int) int {
	if atk == noTypeIndex || def == noTypeIndex {
		return TypeCodeNeutral
	}
	return int(c.data.codes[atk*len(c.data.types)+def])
}

// Code は攻撃タイプ atk が防御タイプ def に対して持つ倍率コード(0/1/2/4)を返す。
// どちらかが TypeNone なら等倍(2)。表がゼロ値なら ErrTypeChartMissing、
// TypeNone でない未知の ID は ErrUnknownType。
func (c TypeChart) Code(atk, def Type) (int, error) {
	if c.data == nil {
		return 0, ErrTypeChartMissing
	}
	ai, err := c.indexOf(atk)
	if err != nil {
		return 0, err
	}
	di, err := c.indexOf(def)
	if err != nil {
		return 0, err
	}
	return c.codeAt(ai, di), nil
}

// Effectiveness は攻撃タイプと防御側タイプ列に対する相性を返す。
// 複合タイプは掛け合わせ、TypeNone の防御タイプは飛ばす。防御タイプが無ければ等倍。
// 表がゼロ値なら ErrTypeChartMissing、未知の ID は ErrUnknownType。
func (c TypeChart) Effectiveness(atk Type, defTypes []Type) (Effectiveness, error) {
	if c.data == nil {
		return Effectiveness{}, ErrTypeChartMissing
	}
	ai, err := c.indexOf(atk)
	if err != nil {
		return Effectiveness{}, err
	}
	num, den := 1, 1
	for _, def := range defTypes {
		di, err := c.indexOf(def)
		if err != nil {
			return Effectiveness{}, err
		}
		if di == noTypeIndex {
			continue
		}
		num *= c.codeAt(ai, di)
		den *= 2
	}
	if ai == noTypeIndex || den == 1 {
		return Effectiveness{Num: 1, Den: 1}, nil // タイプなしの技・防御タイプなしは等倍
	}
	return Effectiveness{Num: num, Den: den}, nil
}

// requireKnown は TypeNone でないタイプがすべて表にあることを確かめる。
// what は失敗時のメッセージに入れる入力の名前。表のゼロ値は ErrTypeChartMissing。
func (c TypeChart) requireKnown(what string, types ...Type) error {
	if c.data == nil {
		return ErrTypeChartMissing
	}
	for _, t := range types {
		if _, err := c.indexOf(t); err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
	}
	return nil
}
