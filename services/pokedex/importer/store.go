package importer

// Store は投入先の DB 操作の抽象(ADR-0104 §10)。CLI のテストでは DB の代わりに偽の実装に
// 差し替える(ネットワーク・DB なしで RunStore の判定を確かめるため)。

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

// ErrSchemaNotReady は data_versions が無い(migrate が済んでいない DB)。
var ErrSchemaNotReady = errors.New("importer: DB のスキーマが未整備(migrate が済んでいない)")

// mysqlErrNoSuchTable は MySQL のエラー番号 1146(テーブルが無い)。
const mysqlErrNoSuchTable = 1146

// Store は AppliedVersions/Apply を抽象化する(RunStore が使う)。
type Store interface {
	AppliedVersions(ctx context.Context) ([]SourceVersion, error)
	Apply(ctx context.Context, out Output, versions []SourceVersion, now time.Time) error
}

// sqlStore は既存の AppliedVersions / Apply(*sql.DB を直接使う関数)を Store として包む。
type sqlStore struct{ db *sql.DB }

// NewSQLStore は db を Store として包む。MySQL のエラー 1146(テーブルが無い = migrate 未実施)は
// ErrSchemaNotReady に包んで返す。
func NewSQLStore(db *sql.DB) Store { return sqlStore{db: db} }

func (s sqlStore) AppliedVersions(ctx context.Context) ([]SourceVersion, error) {
	vs, err := AppliedVersions(ctx, s.db)
	if err != nil {
		return nil, wrapSchemaNotReady(err)
	}
	return vs, nil
}

func (s sqlStore) Apply(ctx context.Context, out Output, versions []SourceVersion, now time.Time) error {
	if err := Apply(ctx, s.db, out, versions, now); err != nil {
		return wrapSchemaNotReady(err)
	}
	return nil
}

// wrapSchemaNotReady は MySQL 1146(テーブルが無い)を ErrSchemaNotReady として包む。
// それ以外のエラーはそのまま返す。
func wrapSchemaNotReady(err error) error {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlErrNoSuchTable {
		return fmt.Errorf("%w: %v", ErrSchemaNotReady, err)
	}
	return err
}

// RunStore は AppliedVersions → NeedsImport(force なら常に投入)→ Apply の順に実行する。
// AppliedVersions が失敗したら Apply を呼ばずにそのエラーを返す(force でも呼ばない。
// 版が読めない = migrate 未実施かもしれない DB にテーブルを作ってしまわないため)。
// NeedsImport が ErrInvalidInput を返す(incoming の形式が不正)ときも Apply を呼ばない。
// 比べる版・記録する版には、取得元の版に変換結果の版(OutputVersion)を足したものを使う
// (取得元が同じでも変換結果が変われば投入する。issue #379・ADR-0122)。
// 取り込んだら true を返す。
func RunStore(ctx context.Context, s Store, out Output, versions []SourceVersion, now time.Time, force bool) (bool, error) {
	applied, err := s.AppliedVersions(ctx)
	if err != nil {
		return false, err
	}
	versions, err = WithOutputVersion(versions, out)
	if err != nil {
		return false, err
	}
	needs, err := NeedsImport(applied, versions)
	if err != nil {
		return false, err
	}
	if !needs && !force {
		return false, nil
	}
	if err := s.Apply(ctx, out, versions, now); err != nil {
		return false, err
	}
	return true, nil
}
