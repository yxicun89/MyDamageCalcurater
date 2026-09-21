// 画面の文言資源(日本語)。タイプの表示名はここに ID → 表示名で持つ(ADR-0016 §4)。
// タイプの一覧(どの ID が存在するか)は相性表のデータ(testdata/golden/typechart.json の types)が正であり、
// ここでは全 ID を過不足なく覆う対応表だけを持つ(コーディング規約 §2: マスタをコードに埋め込まない)。
// TypeId のユニオンは手書きの複製だが、相性表と過不足なく一致することを ja.test.ts が検査して同期を保つ
// (コーディング規約 §2 の「独立した検証」)。

/** 相性表が持つ18タイプの ID(`testdata/golden/typechart.json` の `types` と同じ)。 */
export type TypeId =
  | "bug"
  | "dark"
  | "dragon"
  | "electric"
  | "fairy"
  | "fighting"
  | "fire"
  | "flying"
  | "ghost"
  | "grass"
  | "ground"
  | "ice"
  | "normal"
  | "poison"
  | "psychic"
  | "rock"
  | "steel"
  | "water";

/** タイプ ID → 日本語の表示名(docs/design.md 「タイプ色(自作パレット)」の表と同じ呼び名)。 */
export const typeNameJa: Record<TypeId, string> = {
  normal: "ノーマル",
  fire: "ほのお",
  water: "みず",
  electric: "でんき",
  grass: "くさ",
  ice: "こおり",
  fighting: "かくとう",
  poison: "どく",
  ground: "じめん",
  flying: "ひこう",
  psychic: "エスパー",
  bug: "むし",
  rock: "いわ",
  ghost: "ゴースト",
  dragon: "ドラゴン",
  dark: "あく",
  steel: "はがね",
  fairy: "フェアリー",
};
