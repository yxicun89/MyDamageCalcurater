package master

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"example.com/pokecalc/services/balance/internal/balance"
)

// ErrInvalidTypeChart reports a type chart data file that does not satisfy the schema.
var ErrInvalidTypeChart = errors.New("invalid type chart data")

// embeddedTypeChartJSON is a byte copy of testdata/golden/typechart.json, the type chart
// data introduced by P1-13 (ADR-0013). The copy is checked against the shared file by a test;
// refresh it with `make balance-sync-typechart`.
//
//go:embed data/typechart.json
var embeddedTypeChartJSON []byte

// Type chart codes are the effectiveness ×2 as integers (ADR-0013 §P1-13.1).
const (
	typeChartCodeImmune           = 0
	typeChartCodeNotVeryEffective = 1
	typeChartCodeNeutral          = 2
	typeChartCodeSuperEffective   = 4
)

// typeChartFile mirrors the schema of testdata/golden/typechart.json (schemaVersion 1).
type typeChartFile struct {
	SchemaVersion *int                                      `json:"schemaVersion"`
	Source        string                                    `json:"source"`
	Version       string                                    `json:"version"`
	Generation    int                                       `json:"generation"`
	Note          string                                    `json:"note"`
	ExcludedTypes []string                                  `json:"excludedTypes"`
	Types         []balance.TypeID                          `json:"types"`
	Effectiveness map[balance.TypeID]map[balance.TypeID]int `json:"effectiveness"`
}

// TypeChart is a validated type chart loaded from data. It implements balance.TypeChartProvider.
type TypeChart struct {
	matchups map[[2]balance.TypeID]balance.Multiplier
}

// EmbeddedTypeChart loads the type chart bundled with the service.
func EmbeddedTypeChart() (*TypeChart, error) {
	return LoadTypeChart(bytes.NewReader(embeddedTypeChartJSON))
}

// LoadTypeChart reads and validates a type chart. The type set must be exactly the 18 balance
// types; omitted pairs are neutral; codes other than 0/1/2/4 are rejected. Any violation
// returns an error wrapping ErrInvalidTypeChart and no chart.
func LoadTypeChart(r io.Reader) (*TypeChart, error) {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	var file typeChartFile
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTypeChart, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: data must contain one JSON object", ErrInvalidTypeChart)
	}
	if file.SchemaVersion == nil || *file.SchemaVersion != 1 {
		return nil, fmt.Errorf("%w: schemaVersion must be 1", ErrInvalidTypeChart)
	}
	if err := validateTypeSet(file.Types); err != nil {
		return nil, err
	}

	chart := &TypeChart{matchups: make(map[[2]balance.TypeID]balance.Multiplier)}
	for attack, row := range file.Effectiveness {
		if !attack.Valid() {
			return nil, fmt.Errorf("%w: unknown attack type %q", ErrInvalidTypeChart, attack)
		}
		for defense, code := range row {
			if !defense.Valid() {
				return nil, fmt.Errorf("%w: unknown defense type %q", ErrInvalidTypeChart, defense)
			}
			multiplier, err := multiplierFromCode(code)
			if err != nil {
				return nil, fmt.Errorf("%w: %s→%s: %v", ErrInvalidTypeChart, attack, defense, err)
			}
			chart.matchups[[2]balance.TypeID{attack, defense}] = multiplier
		}
	}
	return chart, nil
}

func validateTypeSet(types []balance.TypeID) error {
	seen := make(map[balance.TypeID]bool, len(types))
	for _, t := range types {
		if !t.Valid() {
			return fmt.Errorf("%w: unknown type %q", ErrInvalidTypeChart, t)
		}
		if seen[t] {
			return fmt.Errorf("%w: duplicate type %q", ErrInvalidTypeChart, t)
		}
		seen[t] = true
	}
	if len(seen) != len(balance.AllTypes()) {
		return fmt.Errorf("%w: types must be exactly the %d balance types", ErrInvalidTypeChart, len(balance.AllTypes()))
	}
	return nil
}

func multiplierFromCode(code int) (balance.Multiplier, error) {
	switch code {
	case typeChartCodeImmune:
		return balance.MultiplierZero, nil
	case typeChartCodeNotVeryEffective:
		return balance.MultiplierHalf, nil
	case typeChartCodeNeutral:
		return balance.MultiplierNormal, nil
	case typeChartCodeSuperEffective:
		return balance.MultiplierDouble, nil
	default:
		return 0, fmt.Errorf("invalid code %d (want 0, 1, 2 or 4)", code)
	}
}

// Matchup implements balance.TypeChartProvider.
func (c *TypeChart) Matchup(attack, defense balance.TypeID) (balance.Multiplier, error) {
	if !attack.Valid() {
		return 0, fmt.Errorf("%w: attack %q", balance.ErrInvalidType, attack)
	}
	if !defense.Valid() {
		return 0, fmt.Errorf("%w: defense %q", balance.ErrInvalidType, defense)
	}
	if multiplier, ok := c.matchups[[2]balance.TypeID{attack, defense}]; ok {
		return multiplier, nil
	}
	return balance.MultiplierNormal, nil
}
