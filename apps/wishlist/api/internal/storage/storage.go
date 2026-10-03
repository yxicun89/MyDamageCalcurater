// Package storage は商品画像の保存(apps/wishlist/CLAUDE.md §2)。初期実装は PVC 上のファイル。
package storage

import (
	"context"
	"errors"
	"io"
)

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
	panic("TODO: storage.ValidName")
}

// FileStorage はディレクトリにファイルとして保存する Storage。
type FileStorage struct {
	dir string
}

var _ Storage = (*FileStorage)(nil)

// NewFileStorage は dir に保存する FileStorage を返す。dir が無ければ作る。
func NewFileStorage(dir string) (*FileStorage, error) {
	panic("TODO: storage.NewFileStorage")
}

func (s *FileStorage) Save(ctx context.Context, r io.Reader) (string, error) {
	panic("TODO: FileStorage.Save")
}

func (s *FileStorage) Open(name string) (io.ReadCloser, string, error) {
	panic("TODO: FileStorage.Open")
}

func (s *FileStorage) Delete(name string) error {
	panic("TODO: FileStorage.Delete")
}
