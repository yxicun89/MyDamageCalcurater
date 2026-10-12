package httpapi

// app・domain ⇄ 生成型の写しの検査(ADR-0811・ADR-0180)。

import (
	"testing"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/domain"
)

// domain.Code は契約の ErrorCode と同じ文字列(fail が文字列のまま写すため)。
func TestDomainCodesAreContractErrorCodes(t *testing.T) {
	for _, c := range domain.AllCodes() {
		if !api.ErrorCode(c).Valid() {
			t.Errorf("domain.Code %q が契約の ErrorCode に無い", c)
		}
	}
}
