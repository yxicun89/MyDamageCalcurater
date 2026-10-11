// G-05(ADR-0339): ポケモンカード。画像かエンブレム(PokemonIcon)+ 名前 + タイプバッジ + 持ち物の印。
// 計算・逆算・構築・素早さ・タイプバランスで共通に使う。onClick があれば全体が 1 つのボタン(押すとピッカーが開く)。
// 名前は中の文字そのもの(画像は装飾)。

import type { ReactElement } from "react";
import { PokemonIcon } from "../images/PokemonIcon";
import { TypeBadge } from "./TypeBadge";

export interface PokemonCardProps {
  readonly speciesKey: string;
  readonly name: string;
  /** タイプ ID(1〜2 個)。最初のタイプがエンブレムの色。 */
  readonly types: readonly string[];
  /** 持ち物の名前(あれば印として出す)。 */
  readonly itemName?: string;
  /** 名前を見出し(h3)にする(領域の見出し h2 の下に置く画面用。既定は文字のまま)。 */
  readonly nameAsHeading?: boolean;
  /** 小さいカード(素早さの縦の並びなど)。 */
  readonly small?: boolean;
  readonly onClick?: () => void;
  readonly className?: string;
  /** -1 にすると Tab の順から外す(onClick があるときだけ効く)。 */
  readonly tabIndex?: number;
}

export function PokemonCard({
  speciesKey,
  name,
  types,
  itemName,
  nameAsHeading = false,
  small = false,
  onClick,
  className,
  tabIndex,
}: PokemonCardProps): ReactElement {
  const classes = ["ui-pokemon-card"];
  if (small) {
    classes.push("ui-pokemon-card--small");
  }
  if (className !== undefined) {
    classes.push(className);
  }
  const content = (
    <>
      <PokemonIcon speciesKey={speciesKey} typeId={types[0]} />
      <span className="ui-pokemon-card__text">
        {nameAsHeading ? (
          <h3 className="ui-pokemon-card__name">{name}</h3>
        ) : (
          <span className="ui-pokemon-card__name">{name}</span>
        )}
        {types.length > 0 && (
          <span className="ui-pokemon-card__types">
            {types.map((type) => (
              <TypeBadge key={type} type={type} />
            ))}
          </span>
        )}
        {itemName !== undefined && itemName !== "" && (
          <span className="ui-pokemon-card__item">{itemName}</span>
        )}
      </span>
    </>
  );
  if (onClick === undefined) {
    return <div className={classes.join(" ")}>{content}</div>;
  }
  return (
    <button type="button" className={classes.join(" ")} tabIndex={tabIndex} onClick={onClick}>
      {content}
    </button>
  );
}
