import { useEffect, useRef } from "react";
import { GenreGlyph, genreColorProps, type GenreGlyphName } from "./icons";

interface Props {
  /** aria-label(常に付ける)。選ばれているときだけ見える文字にもなる */
  name: string;
  icon: GenreGlyphName;
  /** ジャンル色。「すべて」は無し */
  color?: string;
  selected: boolean;
  className?: string;
  glyphSize?: number;
  onClick: () => void;
}

/** ジャンルのボタン。選ばれているものだけ「絵柄 + 名前」の丸薬になる(phase6-spec)。 */
export function GenreButton({ name, icon, color, selected, className = "", glyphSize, onClick }: Props) {
  const ref = useRef<HTMLButtonElement>(null);
  const wasSelected = useRef(selected);
  // 選ばれた(false から true)ときだけ、伸びた丸薬が横に流れる列の端で見切れないよう列の中に入れる。マウント時は動かさない
  useEffect(() => {
    if (selected && !wasSelected.current) {
      // jsdom など matchMedia・scrollIntoView が無い環境でも落ちないよう守る(型は常にあると言うので lint を外す)
      /* eslint-disable @typescript-eslint/no-unnecessary-condition */
      const reduce = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
      ref.current?.scrollIntoView?.({
        block: "nearest",
        inline: "nearest",
        behavior: reduce ? "auto" : "smooth",
      });
      /* eslint-enable @typescript-eslint/no-unnecessary-condition */
    }
    wasSelected.current = selected;
  }, [selected]);

  return (
    <button
      ref={ref}
      type="button"
      className={`genre-button ${className}`.trim()}
      aria-label={name}
      aria-pressed={selected}
      {...(color === undefined ? {} : genreColorProps(color))}
      onClick={onClick}
    >
      <GenreGlyph icon={icon} size={glyphSize} />
      {selected && <span className="genre-name">{name}</span>}
    </button>
  );
}
