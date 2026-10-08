package engine

// 技のフラグ(ADR-0178)。フラグに依存する特性(段階2)の条件に使う。
//
// 値の一覧の正はここ。マスタ(services/internal/master)・migration の CHECK(chk_move_flags_flag)・
// 公開 API の MoveFlag はこれと一致させる。
//
// TODO(ADR-0178 実装): このファイルは spec-writer のスタブ。語彙の定義(定数と AllMoveFlags)だけを置き、
// Known と計算・検証の処理は実装者が書く。

import "errors"

// MoveFlag は技のフラグ1つ。
type MoveFlag string

const (
	// MoveFlagBite はかみつく技(Showdown の flags.bite)。
	MoveFlagBite MoveFlag = "bite"
	// MoveFlagBullet は弾の技(flags.bullet)。
	MoveFlagBullet MoveFlag = "bullet"
	// MoveFlagContact は接触する技(flags.contact)。
	MoveFlagContact MoveFlag = "contact"
	// MoveFlagPulse は波動の技(flags.pulse)。
	MoveFlagPulse MoveFlag = "pulse"
	// MoveFlagPunch はパンチの技(flags.punch)。
	MoveFlagPunch MoveFlag = "punch"
	// MoveFlagRecoil は反動のある技(Showdown の recoil があるか hasCrashDamage が真)。
	MoveFlagRecoil MoveFlag = "recoil"
	// MoveFlagSecondary は追加効果のある技(Showdown の secondary があるか secondaries が空でない)。
	MoveFlagSecondary MoveFlag = "secondary"
	// MoveFlagSlicing は切る技(flags.slicing)。
	MoveFlagSlicing MoveFlag = "slicing"
	// MoveFlagSound は音の技(flags.sound)。
	MoveFlagSound MoveFlag = "sound"
)

// allMoveFlags はフラグの一覧(値の昇順)。
var allMoveFlags = []MoveFlag{
	MoveFlagBite,
	MoveFlagBullet,
	MoveFlagContact,
	MoveFlagPulse,
	MoveFlagPunch,
	MoveFlagRecoil,
	MoveFlagSecondary,
	MoveFlagSlicing,
	MoveFlagSound,
}

// AllMoveFlags はフラグの一覧(値の昇順)のコピーを返す。
func AllMoveFlags() []MoveFlag {
	out := make([]MoveFlag, len(allMoveFlags))
	copy(out, allMoveFlags)
	return out
}

// Known は f が既知のフラグかどうか(大文字小文字を区別する)。
//
// TODO(ADR-0178 実装): スタブ。
func (f MoveFlag) Known() bool {
	return false
}

// ErrInvalidMoveFlags は技のフラグの入力が不正(未知の値・FlagsKnown が偽なのに値がある)。
var ErrInvalidMoveFlags = errors.New("技のフラグが不正")

// PowerConditionMoveFlag は技が Flag を持つ(ADR-0178)。ConditionalPowerMod.Flag を使い、MaxPower 0・MoveType 空。
//
// TODO(ADR-0178 実装): AllPowerConditions に足し、validateStage1・powerConditionHolds で扱う。
const PowerConditionMoveFlag PowerCondition = "move_flag"

// FlagTypeConvert は攻撃側: Flag を持つ技を To タイプにする(うるおいボイス。威力補正なし。ADR-0178)。
type FlagTypeConvert struct {
	Flag MoveFlag
	To   Type
}
