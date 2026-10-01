package readmodel_test

// pokedex export の metadata.json(データの版。issue #108-a・issue #281・issue #403 パッケージ D20・ADR-0128)のテスト。
// balance・speed の loader と schema は未知のフィールドを拒否する(DisallowUnknownFields・additionalProperties: false)ので、
// 既存の4ファイルには欄を足さず、版は別ファイル metadata.json に出す。dataVersion は内部 API
// (GET /internal/pokedex/master)と同じ値(source=version@checksum先頭8桁 の source 昇順の連結)。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/pokedex/internal/httpapi"
	"example.com/pokecalc/services/pokedex/internal/readmodel"
	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

// fixtureDataVersion は storetest.New() の data_versions から作る dataVersion(ADR-0128)。
const fixtureDataVersion = "calc=test-calc-1@22222222," +
	"pokeapi=cafef00dcafef00dcafef00dcafef00dcafef00d@33333333," +
	"showdown=abad1deaabad1deaabad1deaabad1deaabad1dea@11111111"

// metadataFile は metadata.json の形(未知のフィールドは strict で拒否する)。
type metadataFile struct {
	SchemaVersion int    `json:"schemaVersion"`
	DataVersion   string `json:"dataVersion"`
}

func exportMetadata(t *testing.T, q *storetest.Querier) metadataFile {
	t.Helper()
	files, _ := export(t, q)
	var m metadataFile
	strict(t, files.Metadata, &m)
	return m
}

// AC-M1: metadata.json は {"schemaVersion":1,"dataVersion":"<内部 API と同じ形>"} だけ(DB の行の順によらない)。
func TestExportMetadata(t *testing.T) {
	q := storetest.New()
	reverse(q.DataVersions)
	m := exportMetadata(t, q)
	if m.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d, want 1", m.SchemaVersion)
	}
	if m.DataVersion != fixtureDataVersion {
		t.Errorf("dataVersion = %q, want %q", m.DataVersion, fixtureDataVersion)
	}
}

// AC-M2: metadata.json の dataVersion は、同じ DB に対する内部 API の dataVersion と一致する
// (calc-svc と balance・speed の版を同じ値で照合できる。issue #108)。
func TestExportMetadataMatchesMasterAPI(t *testing.T) {
	q := storetest.New()
	rec := httptest.NewRecorder()
	httpapi.NewHandler(q).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/internal/pokedex/master", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("内部 API の status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	var ex struct {
		DataVersion string `json:"dataVersion"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ex); err != nil {
		t.Fatal(err)
	}
	if got := exportMetadata(t, storetest.New()).DataVersion; got != ex.DataVersion {
		t.Errorf("export の dataVersion = %q, 内部 API = %q(同じ値にする)", got, ex.DataVersion)
	}
}

// AC-M3(#281 の回帰テスト): version が "local" 固定の取得元の checksum だけが変わっても dataVersion が変わる。
func TestExportMetadataChangesWithChecksumOnly(t *testing.T) {
	imported := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	withEffects := func(checksum string) *storetest.Querier {
		q := storetest.New()
		q.DataVersions = append(q.DataVersions, store.DataVersion{Source: "effects", Version: "local", Checksum: checksum, ImportedAt: imported})
		return q
	}
	before := exportMetadata(t, withEffects(strings.Repeat("a", 64))).DataVersion
	after := exportMetadata(t, withEffects(strings.Repeat("b", 64))).DataVersion
	if before == after {
		t.Fatalf("effects の checksum だけを変えても dataVersion が変わらない: %q", before)
	}
	if !strings.Contains(after, "effects=local@bbbbbbbb") {
		t.Errorf("dataVersion = %q, want effects=local@bbbbbbbb を含む", after)
	}
}

// AC-M4: data_versions が空(未投入)なら版の無い read model を出さずに失敗し、部分的なファイルを返さない。
func TestExportFailsWithoutDataVersions(t *testing.T) {
	q := storetest.New()
	q.DataVersions = nil
	files, _, err := readmodel.Export(context.Background(), q)
	if !errors.Is(err, readmodel.ErrInvalidExport) {
		t.Fatalf("err = %v, want ErrInvalidExport", err)
	}
	if !reflect.DeepEqual(files, readmodel.Files{}) {
		t.Errorf("失敗なのに Files がゼロ値でない")
	}
}

// AC-M5: 既存の4ファイルは形を変えない(dataVersion の欄を足さない)。balance・speed の今の loader
// (未知のフィールドを拒否する)がそのまま読める。形そのものは readmodel_test.go の strict な各テストが確かめる。
func TestExportKeepsExistingFilesWithoutDataVersion(t *testing.T) {
	files, _ := export(t, storetest.New())
	for name, doc := range map[string][]byte{
		readmodel.FilePokemonTypes: files.PokemonTypes,
		readmodel.FileMoves:        files.Moves,
		readmodel.FileAbilities:    files.Abilities,
		readmodel.FileSpeedPokemon: files.SpeedPokemon,
	} {
		if bytes.Contains(doc, []byte(`"dataVersion"`)) {
			t.Errorf("%s に dataVersion がある(既存の loader が拒否する。metadata.json に出す)", name)
		}
	}
}

// AC-M6: 新しい2ファイルも決定的(DB の行の順によらずバイト単位で同じ)で、数値は整数の字面、末尾は改行1つ。
func TestNewExportFilesAreCanonical(t *testing.T) {
	a, _ := export(t, storetest.New())
	q := storetest.New()
	reverse(q.DataVersions)
	reverse(q.Types)
	reverse(q.TypeChart)
	b, _ := export(t, q)
	plain := regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	for _, tt := range []struct {
		name string
		a, b []byte
	}{
		{readmodel.FileTypeChart, a.TypeChart, b.TypeChart},
		{readmodel.FileMetadata, a.Metadata, b.Metadata},
	} {
		if len(tt.a) == 0 {
			t.Errorf("%s が空", tt.name)
			continue
		}
		if !bytes.Equal(tt.a, tt.b) {
			t.Errorf("%s: 行の順を変えると出力が変わる\n%s\n%s", tt.name, tt.a, tt.b)
		}
		if !bytes.HasSuffix(tt.a, []byte("}\n")) || bytes.HasSuffix(tt.a, []byte("\n\n")) {
			t.Errorf("%s: 末尾が改行1つの正準形でない", tt.name)
		}
		dec := json.NewDecoder(bytes.NewReader(tt.a))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		walkNumbers(v, func(n json.Number) {
			if !plain.MatchString(n.String()) {
				t.Errorf("%s に整数の字面でない数値 %q がある", tt.name, n)
			}
		})
	}
}

// AC-M7: WriteDir は metadata.json を最後に書く。metadata.json が読めた時点で他の5ファイルは同じ export の中身に
// なっている(読む側・版の照合が「版は新、中身は旧」を見ない)。metadata.json を書けなければ失敗を返す。
func TestWriteDirWritesMetadataLast(t *testing.T) {
	files, _ := export(t, storetest.New())
	dir := t.TempDir()
	// metadata.json の場所に中身のあるディレクトリを置き、metadata.json の rename だけが失敗するようにする。
	blocker := filepath.Join(dir, readmodel.FileMetadata)
	if err := os.MkdirAll(filepath.Join(blocker, "keep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteDir(dir); err == nil {
		t.Fatal("metadata.json を書けないのに WriteDir が成功した")
	}
	for name, body := range map[string][]byte{
		readmodel.FilePokemonTypes: files.PokemonTypes,
		readmodel.FileMoves:        files.Moves,
		readmodel.FileAbilities:    files.Abilities,
		readmodel.FileSpeedPokemon: files.SpeedPokemon,
		readmodel.FileTypeChart:    files.TypeChart,
	} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("%s が metadata.json より先に書かれていない: %v", name, err)
			continue
		}
		if !bytes.Equal(got, body) {
			t.Errorf("%s の中身が Files と違う", name)
		}
	}
}
