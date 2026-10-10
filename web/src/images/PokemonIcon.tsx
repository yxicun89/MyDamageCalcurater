// 一覧・選択肢・カードの小さなポケモン表示(G-06。ADR-0343)。画像(thumb)があれば <img>、
// 無い・読めないときはタイプ色のエンブレム。画像は装飾(alt 空)で、名前は隣の文字が担う。
// 画像とエンブレムは同じ寸法の枠(.pokemon-icon)に収め、有無でレイアウトを動かさない。

import { PokemonImage } from "./PokemonImage";
import "./PokemonIcon.css";

// タイプ ID は CSS 変数名に埋め込むので、英小文字・数字・ハイフンだけに限る(typeAccent.ts と同じ)。
const SAFE_TYPE_ID = /^[a-z0-9]+(-[a-z0-9]+)*$/;

interface PokemonIconProps {
  readonly speciesKey: string;
  /** エンブレムの色に使う最初のタイプ。分からなければ省略(中立色)。 */
  readonly typeId?: string;
}

export function PokemonIcon({ speciesKey, typeId }: PokemonIconProps) {
  const color =
    typeId !== undefined && SAFE_TYPE_ID.test(typeId)
      ? `var(--type-${typeId}, var(--text-secondary))`
      : "var(--text-secondary)";
  return (
    <PokemonImage
      speciesKey={speciesKey}
      size="thumb"
      className="pokemon-icon"
      fallback={
        <span
          className="pokemon-icon pokemon-icon--emblem"
          data-testid="pokemon-icon-emblem"
          style={{ backgroundColor: color }}
        />
      }
    />
  );
}
