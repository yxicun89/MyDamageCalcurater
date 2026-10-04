package master

import "example.com/pokecalc/engine"

// ItemRole は持ち物のダメージ計算での役割(ADR-0175 §1)。値は公開 API の ItemRole(api/openapi.yaml)と同じ文字列。
type ItemRole string

const (
	// ItemRoleAttacker は攻撃側で持つとダメージが変わる(または未対応の印が付く)持ち物。
	ItemRoleAttacker ItemRole = "attacker"
	// ItemRoleDefender は防御側で持つとダメージが変わる(または未対応の印が付く)持ち物。
	ItemRoleDefender ItemRole = "defender"
)

// AllItemRoles は役割の全値を、ItemRoles が返す並び(attacker → defender)で返す。
func AllItemRoles() []ItemRole {
	return []ItemRole{ItemRoleAttacker, ItemRoleDefender}
}

// ItemRoles は持ち物の効果とメガストーンかどうかから、ダメージ計算での役割を返す(ADR-0175 §1 の規則の唯一の正)。
// 効果が nil・メガストーンは空配列。戻り値は nil にしない(JSON で [] を出すため)。
func ItemRoles(effect *engine.ItemEffect, isMegaStone bool) []ItemRole {
	roles := []ItemRole{}
	if effect == nil || isMegaStone {
		return roles
	}
	// 0 は engine が「補正なし」と読む値、4096 は ×1.0。どちらもダメージを変えない。
	active := func(mod int) bool { return mod != 0 && mod != engine.Modifier4096 }
	attacker := active(effect.DamageMod) || active(effect.PowerMod) ||
		(effect.BoostType != "" && active(effect.BoostTypeMod)) || effect.UnsupportedAttacker ||
		active(effect.StatMods[engine.StatAtk]) || active(effect.StatMods[engine.StatSpA])
	defender := active(effect.StatMods[engine.StatDef]) || active(effect.StatMods[engine.StatSpD]) ||
		effect.ResistBerryType != "" || effect.UnsupportedDefender
	if attacker {
		roles = append(roles, ItemRoleAttacker)
	}
	if defender {
		roles = append(roles, ItemRoleDefender)
	}
	return roles
}
