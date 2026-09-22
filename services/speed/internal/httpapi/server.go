// Package httpapi は speed サービスの Echo の HTTP アダプタ(ADR-0600 §5)。
package httpapi

import (
	"example.com/pokecalc/services/speed/internal/speed"
	"github.com/labstack/echo/v5"
)

// Dependencies は HTTP アダプタの差し替え可能な境界。
// Pokemon が nil でも起動はし、/healthz は 200、ポケモンを使う API は 503 master_unavailable(ADR-0600 §4)。
type Dependencies struct {
	Pokemon speed.PokemonProvider
}

// New は HTTP ハンドラを返す。
func New(deps Dependencies) *echo.Echo {
	panic("unimplemented")
}
