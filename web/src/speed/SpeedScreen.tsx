// SP3: 素早さ比較の画面(ADR-0604 §4、docs/speed-design.md §5・§6)。
// 左 = 速い順の全体の表(speed-svc の tiers をそのまま描画)、右 = 自分のポケモン(preset / custom / raw)。
// 自分の実数値が左の表のどこに入るかを、同速の段の強調か、段の間の境界線で示す(ADR-0604 §1・§4)。
// 素早さの計算・並び・位置はすべて speed-svc が決める。Web は engine(WASM)・pokedex のマスタを使わない
// (ADR-0604 §5。ScreenProps.engine・master は構造的に無視する)。
//
// 状態の作り(BalanceScreen.tsx と同じ考え方):
//   - pokemon()・table() はマウント時に1回ずつ呼び、それぞれ独立の状態を持つ(片方のエラーがもう片方の
//     表示を消さない。ADR-0604 §4)。
//   - 右の入力(self)から作る PositionRequest の JSON を key にし、key が変わるたびに position() を呼ぶ。
//     応答は .then の中だけで setState し(react-hooks/set-state-in-effect)、cancelled フラグで
//     古い応答を無視する(ADR-0604 §4)。
//   - preset/custom はポケモンを選ぶまで、raw は値を入れるまで呼ばない(mode に要らない項目は送らない。
//     契約上 400 invalid_request になるため)。

import { useEffect, useId, useState, type ReactNode } from "react";
import { speedPresetText, speedScreenText } from "../i18n/ja";
import "./SpeedScreen.css";
import type { components } from "./speed.gen";
import type { SpeedClient, SpeedResult } from "./speedClient";

type Schemas = components["schemas"];

/** 画面の props(App.tsx が app/screens.tsx 経由で注入する。ADR-0604 §2)。 */
export interface SpeedScreenProps {
  readonly speedClient: SpeedClient;
}

/** pokemon()/table() 呼び出し1本の状態(判別 union)。読み込み中は loading のまま表す。 */
type FetchState<T> =
  | { readonly status: "loading" }
  | { readonly status: "success"; readonly value: T }
  | { readonly status: "error"; readonly error: { readonly code: string; readonly message: string } };

/** position() 呼び出し1本の状態。idle は送る入力が無い(呼んでいない。preset/custom は未選択、raw は未入力)。 */
type RequestState<T> =
  | { readonly status: "idle" }
  | { readonly status: "loading" }
  | { readonly status: "success"; readonly value: T }
  | { readonly status: "error"; readonly error: { readonly code: string; readonly message: string } };

/** 直近に届いた position() の応答と、それを生んだ入力の key(CalcScreen.tsx の CompletedCalc と同じ考え方)。 */
interface Completed<T> {
  readonly key: string;
  readonly result: SpeedResult<T>;
}

/** key が今の入力(currentKey)と一致する応答が届いていれば成功/失敗、まだなら loading にする。 */
function deriveRequestState<T>(currentKey: string, completed: Completed<T> | null): RequestState<T> {
  if (completed === null || completed.key !== currentKey) {
    return { status: "loading" };
  }
  return completed.result.ok
    ? { status: "success", value: completed.result.value }
    : { status: "error", error: completed.result.error };
}

/** 自分のポケモンで選べる調整(preset の3つ。scarf は別入力。ADR-0602 §5)。 */
const MINIMAL_PRESET_IDS: readonly Schemas["MinimalPresetId"][] = ["uninvested", "neutral-max", "max"];

/** 性格補正の3値(ADR-0600 §3)。 */
const NATURE_IDS: readonly Schemas["NatureId"][] = ["minus", "neutral", "plus"];

/** 右(自分のポケモン)の入力方法の3値(ADR-0602 §2)。 */
const MODE_IDS: readonly Schemas["PositionRequest"]["mode"][] = ["preset", "custom", "raw"];

/** 右(自分のポケモン)の入力の状態(3つのモードすべての項目を持つが、送るのは mode に要る項目だけ)。 */
interface SelfState {
  readonly mode: Schemas["PositionRequest"]["mode"];
  readonly pokemonId: string;
  readonly preset: Schemas["MinimalPresetId"];
  readonly scarf: boolean;
  readonly sp: number;
  readonly nature: Schemas["NatureId"];
  readonly rank: number;
  /** raw の実数値(コントロール入力のため文字列で持つ。空文字は未入力)。 */
  readonly rawValue: string;
}

/** 既定値(ADR-0604 §4 spec-writer 申し送り: preset:"max"・scarf:false・ポケモン未選択)。 */
function initialSelfState(): SelfState {
  return {
    mode: "preset",
    pokemonId: "",
    preset: "max",
    scarf: false,
    sp: 0,
    nature: "neutral",
    rank: 0,
    rawValue: "",
  };
}

/**
 * self の入力から PositionRequest を作る。mode に要らない項目は含めない(契約上 400 invalid_request の
 * ため)。まだ送れない入力(preset/custom はポケモン未選択、raw は未入力・数でない)は null。
 */
function buildPositionRequest(self: SelfState): Schemas["PositionRequest"] | null {
  if (self.mode === "preset") {
    if (self.pokemonId === "") {
      return null;
    }
    return { mode: "preset", pokemonId: self.pokemonId, preset: self.preset, scarf: self.scarf };
  }
  if (self.mode === "custom") {
    if (self.pokemonId === "") {
      return null;
    }
    return {
      mode: "custom",
      pokemonId: self.pokemonId,
      sp: self.sp,
      nature: self.nature,
      rank: self.rank,
      scarf: self.scarf,
    };
  }
  const trimmed = self.rawValue.trim();
  if (trimmed === "") {
    return null;
  }
  const value = Number(trimmed);
  if (!Number.isFinite(value)) {
    return null;
  }
  return self.pokemonId === "" ? { mode: "raw", value } : { mode: "raw", value, pokemonId: self.pokemonId };
}

/** 数値入力の onChange から整数を読む。空・数でなければ既定値のまま。 */
function parseIntOr(raw: string, fallback: number): number {
  if (raw === "") {
    return fallback;
  }
  const parsed = Number.parseInt(raw, 10);
  return Number.isNaN(parsed) ? fallback : parsed;
}

/** 段の間の境界(faster/slower の件数から決める位置。ADR-0604 §4)。afterSpeed が無ければ表の先頭、
 *  beforeSpeed が無ければ表の末尾。 */
interface Boundary {
  readonly afterSpeed?: number;
  readonly beforeSpeed?: number;
}

/** 境界は「段」ではなく「行」の数で数える(段の entries.length を累計する。ADR-0604 §4)。 */
function computeBoundary(tiers: readonly Schemas["SpeedTier"][], faster: number): Boundary {
  if (faster <= 0) {
    return { beforeSpeed: tiers[0]?.speed };
  }
  let cumulative = 0;
  for (let index = 0; index < tiers.length; index += 1) {
    const tier = tiers[index];
    if (tier === undefined) {
      break;
    }
    cumulative += tier.entries.length;
    if (cumulative >= faster) {
      return { afterSpeed: tier.speed, beforeSpeed: tiers[index + 1]?.speed };
    }
  }
  return { afterSpeed: tiers.at(-1)?.speed };
}

/**
 * 素早さ比較の画面(ADR-0604 §4)。
 */
export function SpeedScreen({ speedClient }: SpeedScreenProps) {
  const [pokemonState, setPokemonState] = useState<FetchState<Schemas["PokemonListResponse"]>>({
    status: "loading",
  });
  const [tableState, setTableState] = useState<FetchState<Schemas["TableResponse"]>>({ status: "loading" });

  // A1/A2: マウント時に pokemon() と table() を1回ずつ呼ぶ(table は presets を省く = 全6行)。
  // 2つは互いに独立(片方のエラーがもう片方の表示を消さない。ADR-0604 §4)。
  useEffect(() => {
    let cancelled = false;
    void speedClient.pokemon().then((result) => {
      if (cancelled) {
        return;
      }
      setPokemonState(
        result.ok ? { status: "success", value: result.value } : { status: "error", error: result.error },
      );
    });
    return () => {
      cancelled = true;
    };
  }, [speedClient]);

  useEffect(() => {
    let cancelled = false;
    void speedClient.table().then((result) => {
      if (cancelled) {
        return;
      }
      setTableState(
        result.ok ? { status: "success", value: result.value } : { status: "error", error: result.error },
      );
    });
    return () => {
      cancelled = true;
    };
  }, [speedClient]);

  const [self, setSelf] = useState<SelfState>(initialSelfState);

  function updateSelf(patch: Partial<SelfState>): void {
    setSelf((current) => ({ ...current, ...patch }));
  }

  const request = buildPositionRequest(self);
  const requestKey = request === null ? "" : JSON.stringify(request);

  const [completedPosition, setCompletedPosition] = useState<Completed<Schemas["PositionResponse"]> | null>(
    null,
  );

  // A5/A6: 入力の変更ごとに position() を呼び直す。key が変わるたびに前の effect の cleanup(cancelled = true)
  // が走るので、古い応答は無視される。
  useEffect(() => {
    if (request === null) {
      return;
    }
    let cancelled = false;
    void speedClient.position(request).then((result) => {
      if (!cancelled) {
        setCompletedPosition({ key: requestKey, result });
      }
    });
    return () => {
      cancelled = true;
    };
    // requestKey が送る内容そのものを表すので、これだけを見る(request 自体を依存に含めると
    // 参照が毎レンダー変わり呼び直しが多重化する)。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [speedClient, requestKey]);

  const positionState: RequestState<Schemas["PositionResponse"]> =
    request === null ? { status: "idle" } : deriveRequestState(requestKey, completedPosition);

  const modeGroupName = useId();
  const presetGroupName = useId();
  const natureGroupName = useId();

  return (
    <div className="speed-screen">
      <section className="speed-table" aria-label={speedScreenText.tableRegionLabel}>
        {tableState.status === "loading" && (
          <p className="speed-screen__notice">{speedScreenText.loadingNotice}</p>
        )}
        {tableState.status === "error" && (
          <p role="alert" className="speed-screen__error">
            {tableState.error.message}
          </p>
        )}
        {tableState.status === "success" && (
          <ul className="speed-table__tiers">{renderTierRows(tableState.value.tiers, positionState)}</ul>
        )}
      </section>

      <section className="speed-self" aria-label={speedScreenText.selfRegionLabel}>
        {pokemonState.status === "error" && (
          <p role="alert" className="speed-screen__error">
            {pokemonState.error.message}
          </p>
        )}

        <div role="radiogroup" aria-label={speedScreenText.modeGroupLabel} className="speed-self__modes">
          {MODE_IDS.map((mode) => {
            const selected = self.mode === mode;
            return (
              <label
                key={mode}
                className={selected ? "speed-self__mode speed-self__mode--selected" : "speed-self__mode"}
              >
                <input
                  type="radio"
                  name={modeGroupName}
                  checked={selected}
                  onChange={() => {
                    updateSelf({ mode });
                  }}
                />
                {speedScreenText.modeLabel[mode]}
              </label>
            );
          })}
        </div>

        <div className="speed-self__field">
          <span>{speedScreenText.pokemonLabel}</span>
          <select
            aria-label={speedScreenText.pokemonLabel}
            value={self.pokemonId}
            onChange={(event) => {
              updateSelf({ pokemonId: event.target.value });
            }}
          >
            <option value="">{speedScreenText.unselectedOption}</option>
            {pokemonState.status === "success" &&
              pokemonState.value.pokemon.map((pokemon) => (
                <option key={pokemon.pokemonId} value={pokemon.pokemonId}>
                  {pokemon.nameJa}
                </option>
              ))}
          </select>
        </div>

        {self.mode === "preset" && (
          <>
            <div
              role="radiogroup"
              aria-label={speedScreenText.presetGroupLabel}
              className="speed-self__field"
            >
              {MINIMAL_PRESET_IDS.map((preset) => {
                const selected = self.preset === preset;
                return (
                  <label key={preset}>
                    <input
                      type="radio"
                      name={presetGroupName}
                      checked={selected}
                      onChange={() => {
                        updateSelf({ preset });
                      }}
                    />
                    {speedPresetText[preset]}
                  </label>
                );
              })}
            </div>
            <label className="speed-self__field">
              <input
                type="checkbox"
                checked={self.scarf}
                onChange={(event) => {
                  updateSelf({ scarf: event.target.checked });
                }}
              />
              {speedScreenText.scarfLabel}
            </label>
          </>
        )}

        {self.mode === "custom" && (
          <>
            <div className="speed-self__field">
              <span>{speedScreenText.spLabel}</span>
              <input
                type="number"
                aria-label={speedScreenText.spLabel}
                min={0}
                max={32}
                value={self.sp}
                onChange={(event) => {
                  updateSelf({ sp: parseIntOr(event.target.value, 0) });
                }}
              />
            </div>
            <div
              role="radiogroup"
              aria-label={speedScreenText.natureGroupLabel}
              className="speed-self__field"
            >
              {NATURE_IDS.map((nature) => {
                const selected = self.nature === nature;
                return (
                  <label key={nature}>
                    <input
                      type="radio"
                      name={natureGroupName}
                      checked={selected}
                      onChange={() => {
                        updateSelf({ nature });
                      }}
                    />
                    {speedScreenText.natureLabel[nature]}
                  </label>
                );
              })}
            </div>
            <div className="speed-self__field">
              <span>{speedScreenText.rankLabel}</span>
              <input
                type="number"
                aria-label={speedScreenText.rankLabel}
                min={-6}
                max={6}
                value={self.rank}
                onChange={(event) => {
                  updateSelf({ rank: parseIntOr(event.target.value, 0) });
                }}
              />
            </div>
            <label className="speed-self__field">
              <input
                type="checkbox"
                checked={self.scarf}
                onChange={(event) => {
                  updateSelf({ scarf: event.target.checked });
                }}
              />
              {speedScreenText.scarfLabel}
            </label>
          </>
        )}

        {self.mode === "raw" && (
          <div className="speed-self__field">
            <span>{speedScreenText.rawValueLabel}</span>
            <input
              type="number"
              aria-label={speedScreenText.rawValueLabel}
              value={self.rawValue}
              onChange={(event) => {
                updateSelf({ rawValue: event.target.value });
              }}
            />
          </div>
        )}

        {positionState.status === "loading" && (
          <p className="speed-screen__notice">{speedScreenText.positionLoadingNotice}</p>
        )}
        {positionState.status === "error" && (
          <p role="alert" className="speed-screen__error">
            {positionState.error.message}
          </p>
        )}
        {positionState.status === "success" && <PositionResult value={positionState.value} />}
      </section>
    </div>
  );
}

/** 左の表の行(段 + 境界の印)。境界は position の応答(tie が無いとき)から作る(ADR-0604 §4)。 */
function renderTierRows(
  tiers: readonly Schemas["SpeedTier"][],
  positionState: RequestState<Schemas["PositionResponse"]>,
): ReactNode[] {
  const selfTieSpeed: number | null =
    positionState.status === "success" && positionState.value.tie.length > 0
      ? positionState.value.speed
      : null;
  const boundary: Boundary | null =
    positionState.status === "success" && positionState.value.tie.length === 0
      ? computeBoundary(tiers, positionState.value.faster)
      : null;

  const rows: ReactNode[] = [];
  if (boundary !== null && boundary.afterSpeed === undefined) {
    rows.push(<BoundaryRow key="speed-boundary" boundary={boundary} />);
  }
  for (const tier of tiers) {
    rows.push(<TierRow key={tier.speed} tier={tier} selfTie={tier.speed === selfTieSpeed} />);
    if (boundary !== null && boundary.afterSpeed === tier.speed) {
      rows.push(<BoundaryRow key="speed-boundary" boundary={boundary} />);
    }
  }
  return rows;
}

interface BoundaryRowProps {
  readonly boundary: Boundary;
}

/** 自分の行が挟まる境界の印(同速が無いとき。ADR-0604 §4)。 */
function BoundaryRow({ boundary }: BoundaryRowProps) {
  const extraProps: Record<string, string> = {};
  if (boundary.afterSpeed !== undefined) {
    extraProps["data-after-speed"] = String(boundary.afterSpeed);
  }
  if (boundary.beforeSpeed !== undefined) {
    extraProps["data-before-speed"] = String(boundary.beforeSpeed);
  }
  return (
    <li className="speed-boundary" data-testid="speed-boundary" {...extraProps}>
      {speedScreenText.selfBoundaryLabel}
    </li>
  );
}

interface TierRowProps {
  readonly tier: Schemas["SpeedTier"];
  /** 自分と同じ実数値の段(position の tie に対応)かどうか。強調のためだけに使う。 */
  readonly selfTie: boolean;
}

/** 段1つ(速い順の表の1行)。2行以上の段には「同速」を出す(Web で並べ替え直さない。ADR-0601 §3)。 */
function TierRow({ tier, selfTie }: TierRowProps) {
  const extraProps: Record<string, string> = {};
  if (selfTie) {
    extraProps["data-self"] = "tie";
  }
  return (
    <li
      className={selfTie ? "speed-tier speed-tier--self" : "speed-tier"}
      data-testid="speed-tier"
      data-speed={tier.speed}
      aria-label={selfTie ? speedScreenText.selfTierLabel : undefined}
      {...extraProps}
    >
      <span className="speed-tier__speed">{speedScreenText.tierSpeedLabel(tier.speed)}</span>
      {tier.entries.length > 1 && <span className="speed-tier__tie">{speedScreenText.tieLabel}</span>}
      <ul className="speed-tier__entries">
        {tier.entries.map((entry, index) => (
          <li
            key={`${entry.pokemonId}-${entry.preset}-${String(index)}`}
            className="speed-entry"
            data-testid="speed-entry"
          >
            <span
              className="speed-entry__emblem"
              data-testid="type-emblem"
              style={{ backgroundColor: `var(--type-${entry.types[0] ?? ""})` }}
            />
            <span>{entry.nameJa}</span>
            <span className="speed-entry__preset">
              {speedScreenText.entrySeparator}
              {speedPresetText[entry.preset]}
            </span>
          </li>
        ))}
      </ul>
    </li>
  );
}

interface PositionResultProps {
  readonly value: Schemas["PositionResponse"];
}

/** 右の結果(実数値・速い/遅い件数・同速の一覧。応答のまま出す。ADR-0604 §4)。 */
function PositionResult({ value }: PositionResultProps) {
  return (
    <div className="speed-self__result">
      <p className="speed-self__speed">{speedScreenText.selfSpeedLabel(value.speed)}</p>
      <p>{speedScreenText.fasterLabel(value.faster)}</p>
      <p>{speedScreenText.slowerLabel(value.slower)}</p>
      <div>
        <h3>{speedScreenText.tieLabel}</h3>
        {value.tie.length === 0 ? (
          <p>{speedScreenText.noTieLabel}</p>
        ) : (
          <ul>
            {value.tie.map((entry, index) => (
              <li key={`${entry.pokemonId}-${entry.preset}-${String(index)}`}>
                {entry.nameJa}
                {speedScreenText.entrySeparator}
                {speedPresetText[entry.preset]}
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
