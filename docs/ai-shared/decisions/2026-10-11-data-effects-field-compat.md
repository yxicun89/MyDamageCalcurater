## 2026-10-11: read model の書き出しは効果定義の未知のフィールドを読み捨てる(ADR-0145)
Decision: `pokedex export` は、特性の効果定義のトップレベルにある未知のフィールド(例 `TypeConvert`)を読み捨てて成功する。読み捨てた名前と件数は標準エラーの警告だけに出す。知っているフィールドの検証は厳格なまま。calc-svc・WASM・importer の厳格なデコードは変えない。
Reason: 固定 digest の古い pokedex イメージが、後から増えた効果で export に失敗し、balance・speed の Pod が Init:Error になった。
Impact: タイプバランス・素早さレーンは、gitops overlay の pokedex イメージの固定 digest を、この修正を含むイメージに上げる。以後、効果定義のフィールドが増えても Init:Error にならない。
read model の JSON(metadata.json 含む)のスキーマは変えていない(balance・speed のローダーが DisallowUnknownFields で厳格なため)。「読み捨てた」印を JSON に入れるのは、両ローダーを寛容にした後の後続。それまでは initContainer のログの警告で気づく。
