package engine

// 技のフラグ(ADR-0178)。フラグに依存する特性(段階2)の条件に使う。
//
// 値の一覧の正はここ。マスタ(services/internal/master)・migration の CHECK(chk_move_flags_flag)・
// 公開 API の MoveFlag はこれと一致させる。

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

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
	return slices.Clone(allMoveFlags)
}

// Known は f が既知のフラグかどうか(大文字小文字を区別する)。
func (f MoveFlag) Known() bool {
	return slices.Contains(allMoveFlags, f)
}

// ErrInvalidMoveFlags は技のフラグの入力が不正(未知の値・FlagsKnown が偽なのに値がある)。
var ErrInvalidMoveFlags = errors.New("技のフラグが不正")

// PowerConditionMoveFlag は技が Flag を持つ(ADR-0178)。ConditionalPowerMod.Flag を使い、MaxPower 0・MoveType 空。
const PowerConditionMoveFlag PowerCondition = "move_flag"

// FlagTypeConvert は攻撃側: Flag を持つ技を To タイプにする(うるおいボイス。威力補正なし。ADR-0178)。
type FlagTypeConvert struct {
	Flag MoveFlag
	To   Type
}

// validateMoveFlags は技のフラグの入力を確かめる: 値はすべて既知で、FlagsKnown が偽なら空。
func validateMoveFlags(m Move) error {
	if !m.FlagsKnown && len(m.Flags) > 0 {
		return fmt.Errorf("%w: FlagsKnown が偽なのに値がある: %v", ErrInvalidMoveFlags, m.Flags)
	}
	for _, f := range m.Flags {
		if !f.Known() {
			return fmt.Errorf("%w: 未知のフラグ %q", ErrInvalidMoveFlags, f)
		}
	}
	return nil
}

// hasFlag は技がフラグ f を持つかを返す。フラグが不明(FlagsKnown が偽)なら持たないものとする。
func (m Move) hasFlag(f MoveFlag) bool {
	return m.FlagsKnown && slices.Contains(m.Flags, f)
}

// attackerDependsOnFlags は攻撃側の特性の効果が技のフラグに依存するか(フラグが不明なときの印に使う。ADR-0178 §5)。
func (e *AbilityEffect) attackerDependsOnFlags() bool {
	if e == nil {
		return false
	}
	if e.FlagTypeConvert != nil {
		return true
	}
	isFlag := func(pm ConditionalPowerMod) bool { return pm.Condition == PowerConditionMoveFlag }
	return slices.ContainsFunc(e.PowerMods, isFlag) || slices.ContainsFunc(e.PostAuraPowerMods, isFlag)
}

// defenderDependsOnFlags は防御側の特性の効果が技のフラグに依存するか(ADR-0178 §5)。
func (e *AbilityEffect) defenderDependsOnFlags() bool {
	return e != nil && (len(e.DefImmuneFlags) > 0 || len(e.DefFinalModsByFlag) > 0)
}

// validateStage2 は特性の段階2の項目(ADR-0178)の値域を確かめる。
func (e AbilityEffect) validateStage2() error {
	if err := validateConditionalPowerMods("PostAuraPowerMods", e.PostAuraPowerMods); err != nil {
		return err
	}
	if c := e.FlagTypeConvert; c != nil {
		if !c.Flag.Known() {
			return fmt.Errorf("FlagTypeConvert.Flag が未知: %q", c.Flag)
		}
		if c.To == TypeNone {
			return errors.New("FlagTypeConvert.To は必須")
		}
	}
	for i, f := range e.DefImmuneFlags {
		if !f.Known() {
			return fmt.Errorf("DefImmuneFlags[%d] が未知: %q", i, f)
		}
		if slices.Contains(e.DefImmuneFlags[:i], f) {
			return fmt.Errorf("DefImmuneFlags に %q が重複している", f)
		}
	}
	// 空の map は整列しない(呼び出しごとの割り当てを避ける)。
	for _, f := range sortedKeysIfAny(e.DefFinalModsByFlag) {
		if !f.Known() {
			return fmt.Errorf("DefFinalModsByFlag のキーが未知: %q", f)
		}
		if err := validateModifier("DefFinalModsByFlag["+string(f)+"]", e.DefFinalModsByFlag[f], false); err != nil {
			return err
		}
	}
	// 空の map は整列しない(呼び出しごとの割り当てを避ける)。
	for _, t := range sortedKeysIfAny(e.DefFinalModsByType) {
		if t == TypeNone {
			return errors.New("DefFinalModsByType のキーが空")
		}
		if err := validateModifier("DefFinalModsByType["+string(t)+"]", e.DefFinalModsByType[t], false); err != nil {
			return err
		}
	}
	return nil
}

// sortedKeysIfAny は map のキーを昇順で返す。空なら割り当てずに nil を返す。
func sortedKeysIfAny[K ~string, V any](m map[K]V) []K {
	if len(m) == 0 {
		return nil
	}
	return slices.Sorted(maps.Keys(m))
}
