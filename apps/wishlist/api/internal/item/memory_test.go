package item_test

import (
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/item/itemtest"
)

func TestMemoryRepositoryContract(t *testing.T) {
	itemtest.RunRepositoryContract(t, func(t *testing.T) item.Repository {
		return item.NewMemoryRepository()
	})
}
