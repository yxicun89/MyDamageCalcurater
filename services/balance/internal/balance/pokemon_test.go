package balance

import (
	"errors"
	"fmt"
	"testing"
)

type stubPokemonTypes map[string][]TypeID

func (s stubPokemonTypes) PokemonTypes(pokemonID string) ([]TypeID, error) {
	if types, ok := s[pokemonID]; ok {
		return types, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrUnknownPokemon, pokemonID)
}

type failingPokemonTypes struct{ err error }

func (f failingPokemonTypes) PokemonTypes(string) ([]TypeID, error) { return nil, f.err }

func TestResolveMembers(t *testing.T) {
	t.Parallel()

	provider := stubPokemonTypes{
		"9001-000": {TypeFire, TypeFlying},
		"9002-000": {TypeGrass},
	}
	tests := []struct {
		name string
		ids  []string
		want []Member
	}{
		{
			name: "single and dual types keep request order",
			ids:  []string{"9002-000", "9001-000"},
			want: []Member{
				{PokemonID: "9002-000", Types: []TypeID{TypeGrass}},
				{PokemonID: "9001-000", Types: []TypeID{TypeFire, TypeFlying}},
			},
		},
		{
			name: "duplicated pokemonId is resolved each time",
			ids:  []string{"9001-000", "9002-000", "9001-000"},
			want: []Member{
				{PokemonID: "9001-000", Types: []TypeID{TypeFire, TypeFlying}},
				{PokemonID: "9002-000", Types: []TypeID{TypeGrass}},
				{PokemonID: "9001-000", Types: []TypeID{TypeFire, TypeFlying}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveMembers(provider, tt.ids)
			if err != nil {
				t.Fatalf("ResolveMembers() error = %v", err)
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("ResolveMembers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveMembersErrors(t *testing.T) {
	t.Parallel()

	providerErr := errors.New("read model broken")
	provider := stubPokemonTypes{"9001-000": {TypeFire, TypeFlying}}
	tests := []struct {
		name     string
		provider PokemonTypeProvider
		ids      []string
		wantErr  error
	}{
		{name: "unknown pokemon", provider: provider, ids: []string{"9999-000"}, wantErr: ErrUnknownPokemon},
		{name: "unknown after known", provider: provider, ids: []string{"9001-000", "9999-000"}, wantErr: ErrUnknownPokemon},
		{name: "nil provider", provider: nil, ids: []string{"9001-000"}, wantErr: ErrNilPokemonTypes},
		{name: "provider error is propagated", provider: failingPokemonTypes{err: providerErr}, ids: []string{"9001-000"}, wantErr: providerErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ResolveMembers(tt.provider, tt.ids)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ResolveMembers() error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
		})
	}
}
