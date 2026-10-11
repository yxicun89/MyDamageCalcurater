//go:build mysql

package store_test

import (
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/adapters/store"
	"example.com/pokecalc/apps/wishlist/api/internal/app/items/itemtest"
)

// フェーズ4-3(docs/phase4-spec.md AC-O4〜O8)。migration 000008 が要る。
func TestMySQLOfficialRepositoryContract(t *testing.T) {
	itemtest.RunOfficialRepositoryContract(t, func(t *testing.T) itemtest.OfficialFullRepository {
		return newMySQLRepo(t).(*store.MySQLRepository)
	})
}
