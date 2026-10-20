package master

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// MetadataFile は read model と同じディレクトリに置かれる export の metadata.json(ADR-0128・ADR-0138)。
const MetadataFile = "metadata.json"

// ErrInvalidMetadata は metadata.json が読めない形(JSON でない・dataVersion が無い)のときのエラー。
var ErrInvalidMetadata = errors.New("invalid read model metadata")

// LoadDataVersionNextTo は readModelPath と同じディレクトリの metadata.json から dataVersion を返す。
// ファイルが無いときは ("", nil)(版不明。古い export でもサービスは動く)。
// あるのに読めない・不正なときはエラー(read model 本体の不正と同じく、呼び出し側は起動を止める)。
func LoadDataVersionNextTo(readModelPath string) (string, error) {
	path := filepath.Join(filepath.Dir(readModelPath), MetadataFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	var meta struct {
		DataVersion string `json:"dataVersion"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", fmt.Errorf("%w: %s: %v", ErrInvalidMetadata, path, err)
	}
	if meta.DataVersion == "" {
		return "", fmt.Errorf("%w: %s: dataVersion is empty", ErrInvalidMetadata, path)
	}
	return meta.DataVersion, nil
}
