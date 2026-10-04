// issue 274(ADR-0312): 計算画面の「詳細」。開閉ボタンと、急所・やけど・天候・フィールド・防御側の壁・攻撃側と防御側のランクの入力。
// 状態は持たない(条件は CalcScreen が持つ。閉じても消さない)。開閉の状態だけここに持つ。閉じている間は中を DOM に出さない。

import { useId, useState, type ReactElement } from "react";
import {
  MAX_RANK,
  MIN_RANK,
  TERRAIN_IDS,
  WEATHER_IDS,
  clampRank,
  formatRank,
  type CalcConditions,
  type EditableDefenderRankStat,
  type EditableRankStat,
  type TerrainId,
  type WeatherId,
} from "../domain/calcConditions";
import type { Screens } from "../engine/types";
import { calcConditionsText } from "../i18n/ja";

interface CalcConditionsPanelProps {
  readonly conditions: CalcConditions;
  readonly onChange: (next: CalcConditions) => void;
  /** 編集するランクのステータス(選択中の技の分類で決まる)。 */
  readonly rankStat: EditableRankStat;
  /** 編集する防御側のランクのステータス(選択中の技の分類で決まる)。 */
  readonly defenderRankStat: EditableDefenderRankStat;
}

interface RadioGroupProps<T extends string> {
  readonly legend: string;
  readonly ids: readonly T[];
  readonly labels: Readonly<Record<T, string>>;
  readonly value: T;
  readonly onChange: (id: T) => void;
}

function RadioGroup<T extends string>({ legend, ids, labels, value, onChange }: RadioGroupProps<T>) {
  const name = useId();
  return (
    <fieldset className="calc-conditions__group">
      <legend className="calc-conditions__legend">{legend}</legend>
      {ids.map((id) => (
        <label key={id} className="calc-conditions__option">
          <input
            type="radio"
            name={name}
            checked={id === value}
            onChange={() => {
              onChange(id);
            }}
          />
          {labels[id]}
        </label>
      ))}
    </fieldset>
  );
}

const SCREEN_KEYS = ["reflect", "lightScreen", "auroraVeil"] as const satisfies ReadonlyArray<keyof Screens>;

export function CalcConditionsPanel({
  conditions,
  onChange,
  rankStat,
  defenderRankStat,
}: CalcConditionsPanelProps): ReactElement {
  const [open, setOpen] = useState(false);
  const bodyId = useId();
  const rank = conditions.ranks[rankStat];
  const defenderRank = conditions.defenderRanks[defenderRankStat];

  function setRank(next: number): void {
    onChange({ ...conditions, ranks: { ...conditions.ranks, [rankStat]: clampRank(next) } });
  }

  function setDefenderRank(next: number): void {
    onChange({
      ...conditions,
      defenderRanks: { ...conditions.defenderRanks, [defenderRankStat]: clampRank(next) },
    });
  }

  return (
    <div className="calc-conditions">
      <button
        type="button"
        className="ui-button ui-button--secondary calc-conditions__toggle"
        aria-expanded={open}
        aria-controls={bodyId}
        onClick={() => {
          setOpen(!open);
        }}
      >
        {calcConditionsText.toggleLabel}
      </button>
      <div id={bodyId}>
        {open && (
          <div className="calc-conditions__body">
            <div className="calc-conditions__group">
              <label className="calc-conditions__option">
                <input
                  type="checkbox"
                  checked={conditions.critical}
                  onChange={(event) => {
                    onChange({ ...conditions, critical: event.target.checked });
                  }}
                />
                {calcConditionsText.criticalLabel}
              </label>
              <label className="calc-conditions__option">
                <input
                  type="checkbox"
                  checked={conditions.burned}
                  onChange={(event) => {
                    onChange({ ...conditions, burned: event.target.checked });
                  }}
                />
                {calcConditionsText.burnLabel}
              </label>
            </div>
            <RadioGroup<WeatherId>
              legend={calcConditionsText.weatherLabel}
              ids={WEATHER_IDS}
              labels={calcConditionsText.weather}
              value={conditions.weather}
              onChange={(weather) => {
                onChange({ ...conditions, weather });
              }}
            />
            <RadioGroup<TerrainId>
              legend={calcConditionsText.terrainLabel}
              ids={TERRAIN_IDS}
              labels={calcConditionsText.terrain}
              value={conditions.terrain}
              onChange={(terrain) => {
                onChange({ ...conditions, terrain });
              }}
            />
            <fieldset className="calc-conditions__group">
              <legend className="calc-conditions__legend">{calcConditionsText.screensLabel}</legend>
              {SCREEN_KEYS.map((key) => (
                <label key={key} className="calc-conditions__option">
                  <input
                    type="checkbox"
                    checked={conditions.defenderScreens[key]}
                    onChange={(event) => {
                      onChange({
                        ...conditions,
                        defenderScreens: { ...conditions.defenderScreens, [key]: event.target.checked },
                      });
                    }}
                  />
                  {calcConditionsText.screens[key]}
                </label>
              ))}
            </fieldset>
            <fieldset className="calc-conditions__group">
              <legend className="calc-conditions__legend">{calcConditionsText.ranksLabel}</legend>
              <button
                type="button"
                className="calc-conditions__rank-button"
                aria-label={calcConditionsText.rankDownLabel}
                disabled={rank <= MIN_RANK}
                onClick={() => {
                  setRank(rank - 1);
                }}
              >
                {calcConditionsText.rankDownSymbol}
              </button>
              <span className="calc-conditions__rank-value">{formatRank(rankStat, rank)}</span>
              <button
                type="button"
                className="calc-conditions__rank-button"
                aria-label={calcConditionsText.rankUpLabel}
                disabled={rank >= MAX_RANK}
                onClick={() => {
                  setRank(rank + 1);
                }}
              >
                {calcConditionsText.rankUpSymbol}
              </button>
            </fieldset>
            <fieldset className="calc-conditions__group">
              <legend className="calc-conditions__legend">{calcConditionsText.defenderRanksLabel}</legend>
              <button
                type="button"
                className="calc-conditions__rank-button"
                aria-label={calcConditionsText.defenderRankDownLabel}
                disabled={defenderRank <= MIN_RANK}
                onClick={() => {
                  setDefenderRank(defenderRank - 1);
                }}
              >
                {calcConditionsText.rankDownSymbol}
              </button>
              <span className="calc-conditions__rank-value">
                {formatRank(defenderRankStat, defenderRank)}
              </span>
              <button
                type="button"
                className="calc-conditions__rank-button"
                aria-label={calcConditionsText.defenderRankUpLabel}
                disabled={defenderRank >= MAX_RANK}
                onClick={() => {
                  setDefenderRank(defenderRank + 1);
                }}
              >
                {calcConditionsText.rankUpSymbol}
              </button>
            </fieldset>
          </div>
        )}
      </div>
    </div>
  );
}
