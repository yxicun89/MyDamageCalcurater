package httpapi

// decodeEffectVerbatim は item_effects / ability_effects の JSON(オブジェクト)を、数値の字面を
// 保ったまま map[string]interface{} に読む(json.Decoder.UseNumber。ADR-0105 §2・ADR-0204 §2)。
// float64 を経由すると 5324.0 → 5324・2^53+1 → 2^53 のように数値が変わり、calc-svc の厳格デコードと
// 共通マスタの整数検査をすり抜けるため。

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func decodeEffectVerbatim(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("効果定義の JSON を読めない: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("効果定義の JSON に後続のデータがある")
	}
	return m, nil
}
