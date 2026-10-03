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

func TestMemoryPriceRepositoryContract(t *testing.T) {
	itemtest.RunPriceRepositoryContract(t, func(t *testing.T) itemtest.FullRepository {
		return item.NewMemoryRepository()
	})
}
