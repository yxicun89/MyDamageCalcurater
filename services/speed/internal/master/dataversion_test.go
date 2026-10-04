package master

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDataVersionNextTo(t *testing.T) {
	t.Parallel()
	valid := `{"schemaVersion":1,"dataVersion":"v1-abcd1234"}`
	broken := `{`
	noVersion := `{"schemaVersion":1}`
	cases := []struct {
		name    string
		content *string
		want    string
		wantErr error
	}{
		{"valid", &valid, "v1-abcd1234", nil},
		{"missing is unknown", nil, "", nil},
		{"broken json", &broken, "", ErrInvalidMetadata},
		{"empty dataVersion", &noVersion, "", ErrInvalidMetadata},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if tc.content != nil {
				if err := os.WriteFile(filepath.Join(dir, MetadataFile), []byte(*tc.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := LoadDataVersionNextTo(filepath.Join(dir, "model.json"))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("dataVersion = %q, want %q", got, tc.want)
			}
		})
	}
}
