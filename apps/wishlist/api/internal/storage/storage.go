// Package storage は商品画像の保存(apps/wishlist/CLAUDE.md §2)。初期実装は PVC 上のファイル。
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var validNameRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\.(jpg|png|webp|gif)$`)

// extByType は内容判定した Content-Type から付ける拡張子。
var extByType = map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif"}

// typeByExt は拡張子から決める Content-Type。
var typeByExt = map[string]string{".jpg": "image/jpeg", ".png": "image/png", ".webp": "image/webp", ".gif": "image/gif"}

// MaxImageBytes は保存できる画像の上限(10 MiB。docs/design.md W-05)。ちょうどは可、1 バイトでも超えれば ErrTooLarge。
const MaxImageBytes = 10 << 20

var (
	// ErrUnsupportedImage は内容の判定が jpeg/png/webp/gif のどれでもないこと(SVG・HTML などは受け付けない)。
	ErrUnsupportedImage = errors.New("storage: unsupported image type")
	// ErrTooLarge は MaxImageBytes を超えたこと。
	ErrTooLarge = errors.New("storage: image too large")
	// ErrNotFound は名前が形式外(UUID v4 + .jpg/.png/.webp/.gif 以外。パストラバーサルを含む)か、ファイルが無いこと。
	ErrNotFound = errors.New("storage: not found")
)

// Storage は画像の保存先。
type Storage interface {
	// Save は r を読み、内容を判定して保存し、新しい名前(UUID v4 + 拡張子。jpeg は .jpg)を返す。
	// 失敗したときは何も残さない。
	Save(ctx context.Context, r io.Reader) (name string, err error)
	// Open は保存した画像と Content-Type(拡張子から決める)を返す。
	Open(name string) (rc io.ReadCloser, contentType string, err error)
	// Delete は画像を消す。形式外の名前は ErrNotFound。形式は正しいがファイルが無いときは nil(冪等)。
	Delete(name string) error
}

// ValidName は name が保存時に付ける形式(小文字 16 進の UUID v4 + .jpg/.png/.webp/.gif)かどうか。
func ValidName(name string) bool {
	return validNameRe.MatchString(name)
}

// FileStorage はディレクトリにファイルとして保存する Storage。
type FileStorage struct {
	dir string
}

var _ Storage = (*FileStorage)(nil)

// NewFileStorage は dir に保存する FileStorage を返す。dir が無ければ作る。
func NewFileStorage(dir string) (*FileStorage, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("storage: 保存先を作れない: %w", err)
	}
	return &FileStorage{dir: dir}, nil
}

func (s *FileStorage) Save(ctx context.Context, r io.Reader) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxImageBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > MaxImageBytes {
		return "", ErrTooLarge
	}
	ext, ok := extByType[http.DetectContentType(data)]
	if !ok {
		return "", ErrUnsupportedImage
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	name := id.String() + ext
	tmp, err := os.CreateTemp(s.dir, ".tmp-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name()) // rename 済みなら失敗するだけ
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(s.dir, name)); err != nil {
		return "", err
	}
	return name, nil
}

func (s *FileStorage) Open(name string) (io.ReadCloser, string, error) {
	if !ValidName(name) {
		return nil, "", ErrNotFound
	}
	f, err := os.Open(filepath.Join(s.dir, name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	return f, typeByExt[strings.ToLower(filepath.Ext(name))], nil
}

func (s *FileStorage) Delete(name string) error {
	if !ValidName(name) {
		return ErrNotFound
	}
	if err := os.Remove(filepath.Join(s.dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
