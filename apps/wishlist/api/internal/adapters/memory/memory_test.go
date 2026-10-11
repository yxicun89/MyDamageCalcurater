package memory_test

import (
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/adapters/memory"
	"example.com/pokecalc/apps/wishlist/api/internal/app/items"
	"example.com/pokecalc/apps/wishlist/api/internal/app/items/itemtest"
)

func TestMemoryRepositoryContract(t *testing.T) {
	itemtest.RunRepositoryContract(t, func(t *testing.T) items.Repository {
		return memory.NewMemoryRepository()
	})
}

func TestMemoryPriceRepositoryContract(t *testing.T) {
	itemtest.RunPriceRepositoryContract(t, func(t *testing.T) itemtest.FullRepository {
		return memory.NewMemoryRepository()
	})
}
