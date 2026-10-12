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
  return (
    <button
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
