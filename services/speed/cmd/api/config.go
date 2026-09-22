package main

import "example.com/pokecalc/services/speed/internal/speed"

// pokemonPathEnv は speed 内の暫定 read model の場所(ADR-0600 §4)。
const pokemonPathEnv = "SPEED_POKEMON_PATH"

// portEnv と defaultPort は待ち受けポート(balance と同じ既定)。
const (
	portEnv     = "PORT"
	defaultPort = "8080"
)

// pokemonProviderFromEnv は SPEED_POKEMON_PATH の read model を読む。
// 未設定または空文字: (nil, nil)(型なしの nil インターフェース。API は 503)。
// 設定されているのに読めない・不正: エラー(main は非 0 で終了する)。
func pokemonProviderFromEnv(lookup func(string) (string, bool)) (speed.PokemonProvider, error) {
	panic("unimplemented")
}

// portFromEnv は PORT を返す。未設定または空文字なら defaultPort。
func portFromEnv(lookup func(string) (string, bool)) string {
	panic("unimplemented")
}
