package readmodel

// MaxCatalogAbilityCount は外部テスト(readmodel_test)から切り詰め件数の上限を読むための別名。
// balance の schema(pokemon-types.schema.json の abilityIds.maxItems)との同期テストが、
// 手で書いた値ではなく実装の定数そのものと比べるために使う(#74)。
const MaxCatalogAbilityCount = maxCatalogAbilityCount
