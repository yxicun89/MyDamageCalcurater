package main

import (
	"example.com/pokecalc/services/speed/internal/master"
	"example.com/pokecalc/services/speed/internal/speed"
)

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
	path, ok := lookup(pokemonPathEnv)
	if !ok || path == "" {
		return nil, nil
	}
	model, err := master.LoadPokemonFile(path)
	if err != nil {
		// 型なしの nil インターフェースを返す(型付きの nil ポインタを非 nil インターフェースに
		// 包んで返す典型的な落とし穴を避ける)。
		return nil, err
	}
	return model, nil
}

// dataVersionFromEnv は SPEED_POKEMON_PATH と同じディレクトリの metadata.json から read model の版を読む
// (ADR-0138)。パス未設定・metadata.json 無しは ("", nil)(版不明)。あるのに不正ならエラー(main は非 0 で終了する)。
func dataVersionFromEnv(lookup func(string) (string, bool)) (string, error) {
	path, ok := lookup(pokemonPathEnv)
	if !ok || path == "" {
		return "", nil
	}
	return master.LoadDataVersionNextTo(path)
}

// portFromEnv は PORT を返す。未設定または空文字なら defaultPort。
func portFromEnv(lookup func(string) (string, bool)) string {
	value, ok := lookup(portEnv)
	if !ok || value == "" {
		return defaultPort
	}
	return value
}
