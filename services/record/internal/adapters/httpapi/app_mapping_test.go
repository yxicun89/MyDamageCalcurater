package httpapi

// 生成型 ⇄ domain の写しの検査(ADR-0811・ADR-0254)。

import (
	"testing"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/record/internal/domain"
)

// domain.Code は契約の ErrorCode と同じ文字列(errFromDomain が文字列のまま写すため)。
func TestDomainCodesAreContractErrorCodes(t *testing.T) {
	for _, c := range domain.AllCodes() {
		if !api.ErrorCode(c).Valid() {
			t.Errorf("domain.Code %q が契約の ErrorCode に無い", c)
		}
	}
}

// contractEnums の省略時の値は、契約の列挙に含まれる(domain が補う既定値が契約の外に出ない)。
func TestContractEnumDefaultsAreValid(t *testing.T) {
	for name, tt := range map[string]struct {
		valid func(string) bool
		v     string
	}{
		"status":  {contractEnums.Status, contractEnums.StatusNone},
		"weather": {contractEnums.Weather, contractEnums.WeatherNone},
		"terrain": {contractEnums.Terrain, contractEnums.TerrainNone},
	} {
		if tt.v == "" || !tt.valid(tt.v) {
			t.Errorf("%s の既定値 %q が契約の列挙に無い", name, tt.v)
		}
	}
}
