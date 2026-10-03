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
import { MIN_RANK, MAX_RANK } from "../domain/calcConditions";
import { MAX_SP_PER_STAT } from "../domain/requests";
import { speedPresetText, speedScreenText } from "../i18n/ja";
import { PokemonImage } from "../images/PokemonImage";
import "./SpeedScreen.css";
import type { components } from "./speed.gen";
import type { SpeedClient, SpeedResult, SpeedTableField } from "./speedClient";

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

/** 左の表の6行すべて(ADR-0601 §2 の順)。絞り込みのチェックボックスと table() の既定に使う。 */
const ALL_PRESET_IDS: readonly Schemas["PresetId"][] = [
  "uninvested",
  "neutral-max",
  "max",
  "max-scarf",
  "max-plus1",
  "max-plus2",
];

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
  /** 自分の追い風・まひ(preset / custom のみ送る。ADR-0607 §1)。 */
  readonly tailwind: boolean;
  readonly paralysis: boolean;
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
    tailwind: false,
    paralysis: false,
    sp: 0,
    nature: "neutral",
    rank: 0,
    rawValue: "",
  };
}

/** 入力欄ごとの範囲外メッセージ(無ければ範囲内)。送信前に画面で止めるための検査(issue 307)。 */
interface FieldErrors {
  readonly sp?: string;
  readonly rank?: string;
  readonly raw?: string;
}

/**
 * 今の mode で使う入力欄だけを検査する(判定画面 JudgeScreen の validationMessage と同じ規則: SP 0〜32、
 * ランク -6〜+6、整数)。空欄の扱い:
 *   - custom の SP・ランクは onChange(parseIntOr)が 0 とみなす(従来どおり。欄は 0 に戻り、エラーにしない)。
 *   - raw の実数値は「未入力」で、呼ばずエラーも出さない。
 * raw の実数値は契約の minimum(1)・整数だけをここで見る。上限は speed サービスが式から導く値で
 * 画面に複製しない(契約の説明)ので、超過は API の 400 を日本語にして出す。
 */
function validateSelf(self: SelfState): FieldErrors {
  if (self.mode === "custom") {
    return {
      sp:
        self.sp < 0 || self.sp > MAX_SP_PER_STAT
          ? speedScreenText.spRangeMessage(MAX_SP_PER_STAT)
          : undefined,
      rank:
        self.rank < MIN_RANK || self.rank > MAX_RANK
          ? speedScreenText.rankRangeMessage(MIN_RANK, MAX_RANK)
          : undefined,
    };
  }
  if (self.mode === "raw") {
    const trimmed = self.rawValue.trim();
    if (trimmed === "") {
      return {};
    }
    const value = Number(trimmed);
    return Number.isInteger(value) && value >= 1 ? {} : { raw: speedScreenText.rawRangeMessage };
  }
  return {};
}

/** サーバーの英語 message は出さず、code から日本語にする(未知の code は汎用の文言)。 */
function errorMessage(error: { readonly code: string }): string {
  const byCode: Readonly<Record<string, string | undefined>> = speedScreenText.errorByCode;
  return byCode[error.code] ?? speedScreenText.errorFallback;
}

/**
 * self の入力から PositionRequest を作る。mode に要らない項目は含めない(契約上 400 invalid_request の
 * ため)。まだ送れない入力(preset/custom はポケモン未選択、raw は未入力・数でない)は null。
 */
function buildPositionRequest(self: SelfState, tableTailwind: boolean): Schemas["PositionRequest"] | null {
  const request = buildBaseRequest(self);
  if (request === null) {
    return null;
  }
  // 省略は false と同じ応答なので、true のものだけを足す(すべて off なら従来と同じ本文。ADR-0607 §1)。
  return {
    ...request,
    ...(request.mode !== "raw" && self.tailwind ? { tailwind: true } : {}),
    ...(request.mode !== "raw" && self.paralysis ? { paralysis: true } : {}),
    ...(tableTailwind ? { tableTailwind: true } : {}),
  };
}

/** 場の効果を除いた、mode ごとの本文(buildPositionRequest が場の効果を足す)。 */
function buildBaseRequest(self: SelfState): Schemas["PositionRequest"] | null {
  const errors = validateSelf(self);
  if (errors.sp !== undefined || errors.rank !== undefined || errors.raw !== undefined) {
    return null;
  }
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

/** 段の間の境界。afterSpeed が無ければ表の先頭、beforeSpeed が無ければ表の末尾。 */
interface Boundary {
  readonly afterSpeed?: number;
  readonly beforeSpeed?: number;
}

/**
 * 境界は、表示中の段(絞り込み後の tiers)の実数値と自分の実数値を直接比べて決める。
 * `faster`/`slower`/`tie` は契約上つねに全6行の表を基準にした値(ADR-0602 §3)なので、絞り込みで表示行数が
 * 減ると基準が合わなくなる。絞り込みの有無によらず正しく引けるよう、tiers の speed だけを見る
 * (Web で自分の素早さを計算し直さない方針は保つ。2026-09-22 critic 指摘で computeBoundary から差し替え)。
 */
function computeBoundary(
  tiers: readonly Schemas["SpeedTier"][],
  ownSpeed: number,
  trickRoom: boolean,
): Boundary {
  // トリックルーム中の表は昇順(遅い順)なので、自分より速い最初の段の前に引く(ADR-0607 §4)。
  const index = tiers.findIndex((tier) => (trickRoom ? tier.speed > ownSpeed : tier.speed < ownSpeed));
  if (index === -1) {
    return { afterSpeed: tiers.at(-1)?.speed };
  }
  return { afterSpeed: tiers[index - 1]?.speed, beforeSpeed: tiers[index]?.speed };
}

/**
 * 素早さ比較の画面(ADR-0604 §4)。
 */
export function SpeedScreen({ speedClient }: SpeedScreenProps) {
  const [pokemonState, setPokemonState] = useState<FetchState<Schemas["PokemonListResponse"]>>({
    status: "loading",
  });
  // 左の表の絞り込み(道具・ランク。ユーザー確定仕様。docs/plan.md「SP: 素早さ比較」・ADR-0601 §4)。
  // 既定は全6行選択(= 絞り込みなし)。絞り込みを変えるたびに table() を呼び直すので、position() と同じ
  // key(presetsKey)方式にする(effect の中で setState を同期的に呼ばない。react-hooks/set-state-in-effect)。
  const [selectedPresets, setSelectedPresets] = useState<ReadonlySet<Schemas["PresetId"]>>(
    () => new Set(ALL_PRESET_IDS),
  );

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

  // 選択された行だけを ADR-0601 §2 の順で並べたもの。全6行選択なら table() には省略して渡す
  // (presets: [] は契約上 400 invalid_request になるため、絞り込み UI 側で最低1つを保証する)。
  const activePresets = ALL_PRESET_IDS.filter((id) => selectedPresets.has(id));
  // 表の場の状態(追い風・トリックルーム。ADR-0607)。表の呼び直しの key に含める。
  const [tableTailwind, setTableTailwind] = useState(false);
  const [trickRoom, setTrickRoom] = useState(false);
  const presetsKey = `${activePresets.join(",")}|${String(tableTailwind)}|${String(trickRoom)}`;

  const [completedTable, setCompletedTable] = useState<Completed<Schemas["TableResponse"]> | null>(null);

  useEffect(() => {
    let cancelled = false;
    const presetsArg = activePresets.length === ALL_PRESET_IDS.length ? undefined : activePresets;
    // すべて off なら第2引数を渡さない(従来と同じ呼び出し)。
    const fieldArg: SpeedTableField | undefined =
      tableTailwind || trickRoom ? { tailwind: tableTailwind, trickRoom } : undefined;
    void speedClient.table(presetsArg, fieldArg).then((result) => {
      if (!cancelled) {
        setCompletedTable({ key: presetsKey, result });
      }
    });
    return () => {
      cancelled = true;
    };
    // presetsKey が絞り込みの内容そのものを表すので、これだけを見る(activePresets は毎レンダー新しい
    // 配列参照になるため依存に含めない。position の requestKey と同じ考え方)。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [speedClient, presetsKey]);

  // table() は常に何らかの presets で呼ぶので idle にはならない(位置の request===null に相当するものが無い)。
  const tableState: RequestState<Schemas["TableResponse"]> = deriveRequestState(presetsKey, completedTable);

  /** 絞り込みのチェックボックスの切り替え。最後の1つは外せない(契約上 presets は1つ以上)。 */
  function toggleFilterPreset(id: Schemas["PresetId"]): void {
    setSelectedPresets((current) => {
      if (current.has(id) && current.size === 1) {
        return current;
      }
      const next = new Set(current);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  const [self, setSelf] = useState<SelfState>(initialSelfState);

  function updateSelf(patch: Partial<SelfState>): void {
    setSelf((current) => ({ ...current, ...patch }));
  }

  const fieldErrors = validateSelf(self);
  const request = buildPositionRequest(self, tableTailwind);
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
  const filterMinimumNoticeId = useId();
  const spErrorId = useId();
  const rankErrorId = useId();
  const rawErrorId = useId();

  return (
    <div className="speed-screen">
      <section className="speed-table" aria-label={speedScreenText.tableRegionLabel}>
        <div
          role="group"
          aria-label={speedScreenText.filterGroupLabel}
          className="speed-table__filter"
          data-testid="speed-filter"
        >
          {ALL_PRESET_IDS.map((id) => {
            const checked = selectedPresets.has(id);
            const lastOne = checked && selectedPresets.size === 1;
            return (
              <label key={id} className="speed-table__filter-option">
                <input
                  type="checkbox"
                  checked={checked}
                  aria-disabled={lastOne}
                  aria-describedby={lastOne ? filterMinimumNoticeId : undefined}
                  onChange={() => {
                    // aria-disabled はキーボード操作を止めないため、切り替え側(toggleFilterPreset)にも
                    // 「最後の1つは外せない」ガードを持たせている(二重のガード)。
                    toggleFilterPreset(id);
                  }}
                />
                {speedPresetText[id]}
              </label>
            );
          })}
          {selectedPresets.size === 1 && (
            <p id={filterMinimumNoticeId} className="speed-screen__notice">
              {speedScreenText.filterMinimumNotice}
            </p>
          )}
        </div>

        <div role="group" aria-label={speedScreenText.fieldGroupLabel} className="speed-table__filter">
          <label className="speed-table__filter-option">
            <input
              type="checkbox"
              checked={tableTailwind}
              onChange={(event) => {
                setTableTailwind(event.target.checked);
              }}
            />
            {speedScreenText.tableTailwindLabel}
          </label>
          <label className="speed-table__filter-option">
            <input
              type="checkbox"
              checked={trickRoom}
              onChange={(event) => {
                setTrickRoom(event.target.checked);
              }}
            />
            {speedScreenText.trickRoomLabel}
          </label>
        </div>

        {tableState.status === "loading" && (
          <p className="speed-screen__notice">{speedScreenText.loadingNotice}</p>
        )}
        {tableState.status === "error" && (
          <p role="alert" className="speed-screen__error">
            {errorMessage(tableState.error)}
          </p>
        )}
        {tableState.status === "success" && (
          <ul className="speed-table__tiers">
            {renderTierRows(tableState.value.tiers, positionState, trickRoom)}
          </ul>
        )}
      </section>

      <section className="speed-self" aria-label={speedScreenText.selfRegionLabel}>
        {pokemonState.status === "error" && (
          <p role="alert" className="speed-screen__error">
            {errorMessage(pokemonState.error)}
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
            <SelfFieldChecks self={self} onChange={updateSelf} />
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
                max={MAX_SP_PER_STAT}
                aria-invalid={fieldErrors.sp !== undefined}
                aria-describedby={fieldErrors.sp === undefined ? undefined : spErrorId}
                value={self.sp}
                onChange={(event) => {
                  updateSelf({ sp: parseIntOr(event.target.value, 0) });
                }}
              />
              {fieldErrors.sp !== undefined && (
                <p id={spErrorId} role="alert" className="speed-screen__error">
                  {fieldErrors.sp}
                </p>
              )}
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
                min={MIN_RANK}
                max={MAX_RANK}
                aria-invalid={fieldErrors.rank !== undefined}
                aria-describedby={fieldErrors.rank === undefined ? undefined : rankErrorId}
                value={self.rank}
                onChange={(event) => {
                  updateSelf({ rank: parseIntOr(event.target.value, 0) });
                }}
              />
              {fieldErrors.rank !== undefined && (
                <p id={rankErrorId} role="alert" className="speed-screen__error">
                  {fieldErrors.rank}
                </p>
              )}
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
            <SelfFieldChecks self={self} onChange={updateSelf} />
          </>
        )}

        {self.mode === "raw" && (
          <div className="speed-self__field">
            <span>{speedScreenText.rawValueLabel}</span>
            <input
              type="number"
              aria-label={speedScreenText.rawValueLabel}
              min={1}
              aria-invalid={fieldErrors.raw !== undefined}
              aria-describedby={fieldErrors.raw === undefined ? undefined : rawErrorId}
              value={self.rawValue}
              onChange={(event) => {
                updateSelf({ rawValue: event.target.value });
              }}
            />
            {fieldErrors.raw !== undefined && (
              <p id={rawErrorId} role="alert" className="speed-screen__error">
                {fieldErrors.raw}
              </p>
            )}
          </div>
        )}

        {positionState.status === "loading" && (
          <p className="speed-screen__notice">{speedScreenText.positionLoadingNotice}</p>
        )}
        {positionState.status === "error" && (
          <p role="alert" className="speed-screen__error">
            {errorMessage(positionState.error)}
          </p>
        )}
        {positionState.status === "success" && (
          <PositionResult value={positionState.value} trickRoom={trickRoom} />
        )}
      </section>
    </div>
  );
}

/**
 * 左の表の行(段 + 境界の印)。強調・境界は表示中(絞り込み後)の tiers の実数値と自分の実数値を直接比べて
 * 決める(`positionState.value.tie`・`faster` は全6行基準なので、絞り込みで表示行数が変わると使えない。
 * 2026-09-22 critic 指摘で修正)。同じ実数値の段が表示されていればそれを強調し、無ければ境界を引く。
 */
function renderTierRows(
  tiers: readonly Schemas["SpeedTier"][],
  positionState: RequestState<Schemas["PositionResponse"]>,
  trickRoom: boolean,
): ReactNode[] {
  const ownSpeed: number | null = positionState.status === "success" ? positionState.value.speed : null;
  const selfTieSpeed: number | null =
    ownSpeed !== null && tiers.some((tier) => tier.speed === ownSpeed) ? ownSpeed : null;
  const boundary: Boundary | null =
    ownSpeed !== null && selfTieSpeed === null ? computeBoundary(tiers, ownSpeed, trickRoom) : null;

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
      {...extraProps}
    >
      {selfTie && <span className="speed-tier__self-label">{speedScreenText.selfTierLabel}</span>}
      <span className="speed-tier__speed">{speedScreenText.tierSpeedLabel(tier.speed)}</span>
      {tier.entries.length > 1 && <span className="speed-tier__tie">{speedScreenText.tieLabel}</span>}
      <ul className="speed-tier__entries">
        {tier.entries.map((entry, index) => (
          <li
            key={`${entry.pokemonId}-${entry.preset}-${String(index)}`}
            className="speed-entry"
            data-testid="speed-entry"
          >
            <PokemonImage
              speciesKey={entry.pokemonId}
              size="thumb"
              className="speed-entry__image"
              fallback={
                <span
                  className="speed-entry__emblem"
                  data-testid="type-emblem"
                  style={{ backgroundColor: `var(--type-${entry.types[0] ?? ""})` }}
                />
              }
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

interface SelfFieldChecksProps {
  readonly self: SelfState;
  readonly onChange: (patch: Partial<SelfState>) => void;
}

/** 自分の追い風・まひ(preset / custom のみ。raw は補正済みの値を入れるので出さない。ADR-0607 §1)。 */
function SelfFieldChecks({ self, onChange }: SelfFieldChecksProps) {
  return (
    <>
      <label className="speed-self__field">
        <input
          type="checkbox"
          checked={self.tailwind}
          onChange={(event) => {
            onChange({ tailwind: event.target.checked });
          }}
        />
        {speedScreenText.selfTailwindLabel}
      </label>
      <label className="speed-self__field">
        <input
          type="checkbox"
          checked={self.paralysis}
          onChange={(event) => {
            onChange({ paralysis: event.target.checked });
          }}
        />
        {speedScreenText.paralysisLabel}
      </label>
    </>
  );
}

interface PositionResultProps {
  readonly value: Schemas["PositionResponse"];
  /** トリックルーム中は faster/slower を行動順に読み替えた行を足す(ADR-0607 §4)。 */
  readonly trickRoom: boolean;
}

/** 右の結果(実数値・速い/遅い件数・同速の一覧。応答のまま出す。ADR-0604 §4)。 */
function PositionResult({ value, trickRoom }: PositionResultProps) {
  return (
    <div className="speed-self__result">
      <p className="speed-self__speed">{speedScreenText.selfSpeedLabel(value.speed)}</p>
      <p>{speedScreenText.fasterLabel(value.faster)}</p>
      <p>{speedScreenText.slowerLabel(value.slower)}</p>
      {trickRoom && (
        <>
          <p>{speedScreenText.movesBeforeLabel(value.slower)}</p>
          <p>{speedScreenText.movesAfterLabel(value.faster)}</p>
        </>
      )}
      <div>
        {/* BalanceScreen と同じく見出し要素は使わない(h1 の直下でレベルを飛ばさないため)。 */}
        <p className="speed-self__result-label">{speedScreenText.tieLabel}</p>
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
