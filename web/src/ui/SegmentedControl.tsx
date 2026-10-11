// G-05(ADR-0339、docs/design.md「部品の一覧」): 区切りボタン。2〜4 択の排他。ラジオの作法(radiogroup + radio)で、
// 矢印キーで選択とフォーカスが一緒に動く(roving tabindex)。選択中は塗り + チェックの印(色だけに頼らない)。

import { useRef, type KeyboardEvent, type ReactElement } from "react";
import { Icon, type IconName } from "./Icon";

export interface SegmentedOption<T extends string> {
  readonly value: T;
  /** 見える名前(読み上げ名と同じ)。 */
  readonly label: string;
  readonly icon?: IconName;
  readonly disabled?: boolean;
}

export interface SegmentedControlProps<T extends string> {
  /** 全体の名前(見えるラベルと同じ語)。 */
  readonly label: string;
  readonly options: readonly SegmentedOption<T>[];
  readonly value: T;
  readonly onChange: (value: T) => void;
  readonly disabled?: boolean;
  /** 選べない選択肢がある理由など、説明の要素の id(aria-describedby)。 */
  readonly describedBy?: string;
  readonly className?: string;
}

const MIN_OPTIONS = 2;
const MAX_OPTIONS = 4;

export function SegmentedControl<T extends string>({
  label,
  options,
  value,
  onChange,
  disabled = false,
  describedBy,
  className,
}: SegmentedControlProps<T>): ReactElement {
  if (options.length < MIN_OPTIONS || options.length > MAX_OPTIONS) {
    throw new Error(
      `SegmentedControl の選択肢は ${String(MIN_OPTIONS)}〜${String(MAX_OPTIONS)} 個(5 択以上はチップ)`,
    );
  }
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  const isEnabled = (index: number): boolean => {
    const option = options[index];
    return option !== undefined && !disabled && option.disabled !== true;
  };
  const selectedIndex = options.findIndex((option) => option.value === value);
  // 選択中が選べない・無いときは、最初の選べる選択肢を tab の入口にする。
  const tabStop = isEnabled(selectedIndex) ? selectedIndex : options.findIndex((_, i) => isEnabled(i));

  function move(from: number, step: 1 | -1): void {
    for (let n = 1; n <= options.length; n += 1) {
      const index = (from + step * n + options.length * n) % options.length;
      const option = options[index];
      if (option !== undefined && isEnabled(index)) {
        onChange(option.value);
        refs.current[index]?.focus();
        return;
      }
    }
  }

  function edge(index: number): void {
    const option = options[index];
    if (option !== undefined && isEnabled(index)) {
      onChange(option.value);
      refs.current[index]?.focus();
    }
  }

  function onKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number): void {
    switch (event.key) {
      case "ArrowRight":
      case "ArrowDown":
        event.preventDefault();
        move(index, 1);
        break;
      case "ArrowLeft":
      case "ArrowUp":
        event.preventDefault();
        move(index, -1);
        break;
      case "Home":
        event.preventDefault();
        edge(0);
        break;
      case "End":
        event.preventDefault();
        edge(options.length - 1);
        break;
      default:
    }
  }

  return (
    <div
      role="radiogroup"
      aria-label={label}
      aria-describedby={describedBy}
      className={className === undefined ? "ui-segmented" : `ui-segmented ${className}`}
    >
      {options.map((option, index) => {
        const selected = option.value === value;
        return (
          <button
            key={option.value}
            ref={(element) => {
              refs.current[index] = element;
            }}
            type="button"
            role="radio"
            aria-checked={selected}
            tabIndex={index === tabStop ? 0 : -1}
            disabled={!isEnabled(index)}
            className={`ui-segmented__option${selected ? " ui-segmented__option--selected" : ""}`}
            onClick={() => {
              onChange(option.value);
            }}
            onKeyDown={(event) => {
              onKeyDown(event, index);
            }}
          >
            {selected ? (
              <Icon name="check" size={16} />
            ) : option.icon === undefined ? null : (
              <Icon name={option.icon} size={16} />
            )}
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
