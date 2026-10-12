// I-speed-4b(ADR-0610 §4・§7): 素早さ画面。「相手を選んで、先に動くための最低ライン」と「自分の素早さのこれ以上」を出す。
// 入力(自分 / 相手 / 場)→ 出力 1(想定ごとのカード)→ 出力 2(「これ以上」)。素早さの値・区間はすべて speed-svc が決め、
// Web は計算し直さない。表(ADR-0608)・周辺パネル(ADR-0609)は出さない。engine(WASM)・マスタは使わない(ADR-0604 §5)。

import { useEffect, useId, useMemo, useState, type ReactElement } from "react";
import { MAX_RANK, MIN_RANK } from "../domain/calcConditions";
import { MAX_SP_PER_STAT } from "../domain/requests";
import { rankStatNameJa } from "../i18n/common";
import { speedPresetText, speedScreenText } from "../i18n/ja";
import { ToggleChip } from "../screens/ToggleChip";
import { PokemonCard } from "../ui/PokemonCard";
import { RankMeter } from "../ui/RankMeter";
import { SegmentedControl } from "../ui/SegmentedControl";
import { Stepper } from "../ui/Stepper";
import { typeAccentStyle } from "../ui/typeAccent";
import "./SpeedScreen.css";
import type { components } from "./speed.gen";
import type { SpeedClient, SpeedResult } from "./speedClient";

type Schemas = components["schemas"];
type PresetId = Schemas["PresetId"];
type NatureId = Schemas["NatureId"];
type SpeedPokemon = Schemas["SpeedPokemon"];

/**
 * 「この相手で連続技を計算」で計算画面の連続技へ渡す内容(ADR-0610 §4・§6 Q7)。
 * speed 画面はダメージ・確定数を出さない。今の振り(speed)があれば素早さの SP・性格・ランク・スカーフも渡す。
 */
export interface SpeedComboRequest {
  /** 攻撃側 = 自分のポケモン。 */
  readonly attackerPokemonId: string;
  /** 防御側 = 相手のポケモン。 */
  readonly defenderPokemonId: string;
  /** 「今の振り」を入れているときだけ。 */
  readonly speed?: {
    readonly sp: number;
    readonly nature: Schemas["NatureId"];
    readonly rank: number;
    readonly scarf: boolean;
  };
}

/** 画面の props(App.tsx が app/screens.tsx 経由で注入する。ADR-0604 §2)。 */
export interface SpeedScreenProps {
  readonly speedClient: SpeedClient;
  /**
   * 連続技の画面へ移る口。連続技の画面が画面レジストリ(ADR-0323)に載るまでは渡されず、
   * 渡されないときは「この相手で連続技を計算」のリンクを出さない(無効表示もしない)。
   */
  readonly onOpenCombo?: (request: SpeedComboRequest) => void;
}

/** 表の順(ADR-0601 §2)の 6 プリセット。想定のチップと「これ以上」の並び。 */
const PRESET_ORDER: readonly PresetId[] = [
  "uninvested",
  "neutral-max",
  "max",
  "max-scarf",
  "max-plus1",
  "max-plus2",
];
/** 想定の既定(ADR-0610 §2。+1・+2 は足せる)。 */
const DEFAULT_ASSUMPTIONS: readonly PresetId[] = ["uninvested", "neutral-max", "max", "max-scarf"];
const NATURES: readonly NatureId[] = ["minus", "neutral", "plus"];

/** 非同期の応答の置き場。key = その応答を求めた入力。今の入力の key と違えば古い(捨てる)。 */
interface Settled<T> {
  readonly key: string;
  readonly result: SpeedResult<T>;
}

/** code から日本語の文言にする(サーバーの英語 message は出さない。issue 307)。 */
function errorText(code: string): string {
  const messages: Readonly<Record<string, string>> = speedScreenText.errorByCode;
  return Object.hasOwn(messages, code)
    ? (messages[code] ?? speedScreenText.errorFallback)
    : speedScreenText.errorFallback;
}

/** 入力が変わるたびに呼び直し、古い応答は捨てる(key 方式)。key が null なら呼ばない。 */
function useSettled<T>(
  key: string | null,
  call: (request: unknown) => Promise<SpeedResult<T>>,
): {
  readonly loading: boolean;
  readonly result: SpeedResult<T> | null;
} {
  const [settled, setSettled] = useState<Settled<T> | null>(null);
  useEffect(() => {
    if (key === null) {
      return;
    }
    let cancelled = false;
    void call(JSON.parse(key) as unknown).then((result) => {
      if (!cancelled) {
        setSettled({ key, result });
      }
    });
    return () => {
      cancelled = true;
    };
    // call は呼び出し側で安定させている(client のメソッド)。key が変わったときだけ呼び直す。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  if (key === null) {
    return { loading: false, result: null };
  }
  return settled?.key === key ? { loading: false, result: settled.result } : { loading: true, result: null };
}

export function SpeedScreen({ speedClient, onOpenCombo }: SpeedScreenProps): ReactElement {
  const [list, setList] = useState<SpeedResult<Schemas["PokemonListResponse"]> | null>(null);
  const [selfId, setSelfId] = useState("");
  const [opponentId, setOpponentId] = useState("");
  const [rank, setRank] = useState(0);
  const [scarf, setScarf] = useState(false);
  const [selfTailwind, setSelfTailwind] = useState(false);
  const [selfParalysis, setSelfParalysis] = useState(false);
  const [useCurrent, setUseCurrent] = useState(false);
  const [sp, setSp] = useState(0);
  const [nature, setNature] = useState<NatureId>("neutral");
  const [assumptions, setAssumptions] = useState<ReadonlySet<PresetId>>(new Set(DEFAULT_ASSUMPTIONS));
  const [minimumNotice, setMinimumNotice] = useState(false);
  const [opponentTailwind, setOpponentTailwind] = useState(false);
  const [opponentParalysis, setOpponentParalysis] = useState(false);
  const [trickRoom, setTrickRoom] = useState(false);
  const ids = useId();

  useEffect(() => {
    let cancelled = false;
    void speedClient.pokemon().then((result) => {
      if (!cancelled) {
        setList(result);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [speedClient]);

  const pokemon: readonly SpeedPokemon[] = list?.ok === true ? list.value.pokemon : [];
  const selfPokemon = pokemon.find((entry) => entry.pokemonId === selfId);
  const opponentPokemon = pokemon.find((entry) => entry.pokemonId === opponentId);

  // ---- 出力 1: matchup() ----
  const matchupKey = useMemo(() => {
    if (selfId === "" || opponentId === "") {
      return null;
    }
    const request: Schemas["MatchupRequest"] = {
      self: {
        pokemonId: selfId,
        ...(rank === 0 ? {} : { rank }),
        ...(scarf ? { scarf } : {}),
        ...(selfTailwind ? { tailwind: true } : {}),
        ...(selfParalysis ? { paralysis: true } : {}),
        ...(useCurrent ? { current: { sp, nature } } : {}),
      },
      opponent: {
        pokemonId: opponentId,
        assumptions: PRESET_ORDER.filter((preset) => assumptions.has(preset)).map((preset) => ({ preset })),
        ...(opponentTailwind ? { tailwind: true } : {}),
        ...(opponentParalysis ? { paralysis: true } : {}),
      },
      ...(trickRoom ? { trickRoom: true } : {}),
    };
    return JSON.stringify(request);
  }, [
    selfId,
    opponentId,
    rank,
    scarf,
    selfTailwind,
    selfParalysis,
    useCurrent,
    sp,
    nature,
    assumptions,
    opponentTailwind,
    opponentParalysis,
    trickRoom,
  ]);
  const matchup = useSettled<Schemas["MatchupResponse"]>(matchupKey, (request) =>
    speedClient.matchup(request as Schemas["MatchupRequest"]),
  );

  // ---- 出力 2: reach()(今の振りがあるとき。相手は不要。トリックルームは本文に入れない) ----
  const reachKey = useMemo(() => {
    if (selfId === "" || !useCurrent) {
      return null;
    }
    const request: Schemas["PositionRequest"] = {
      mode: "custom",
      pokemonId: selfId,
      sp,
      nature,
      rank,
      scarf,
      ...(selfTailwind ? { tailwind: true } : {}),
      ...(selfParalysis ? { paralysis: true } : {}),
      ...(opponentTailwind ? { tableTailwind: true } : {}),
    };
    return JSON.stringify(request);
  }, [selfId, useCurrent, sp, nature, rank, scarf, selfTailwind, selfParalysis, opponentTailwind]);
  const reach = useSettled<Schemas["ReachResponse"]>(reachKey, (request) =>
    speedClient.reach(request as Schemas["PositionRequest"]),
  );

  function toggleAssumption(preset: PresetId, checked: boolean): void {
    if (!checked && assumptions.size <= 1) {
      setMinimumNotice(true);
      return;
    }
    setMinimumNotice(false);
    const next = new Set(assumptions);
    if (checked) {
      next.add(preset);
    } else {
      next.delete(preset);
    }
    setAssumptions(next);
  }

  const matchupHeading = trickRoom ? speedScreenText.matchupHeadingTrickRoom : speedScreenText.matchupHeading;
  const selfHeadingId = `${ids}-self`;
  const opponentHeadingId = `${ids}-opponent`;
  const matchupHeadingId = `${ids}-matchup`;
  const reachHeadingId = `${ids}-reach`;

  return (
    <div className="speed-screen">
      {list?.ok === false && (
        <p role="alert" className="speed-screen__error">
          {errorText(list.error.code)}
        </p>
      )}
      {list === null && <p className="speed-screen__notice">{speedScreenText.pokemonListLoadingNotice}</p>}

      <div className="speed-screen__inputs">
        <section
          aria-labelledby={selfHeadingId}
          className="speed-card speed-card--typed"
          style={typeAccentStyle(selfPokemon?.types[0])}
        >
          <h2 id={selfHeadingId} className="speed-card__region">
            {speedScreenText.selfRegionLabel}
          </h2>
          <PokemonSelect
            label={speedScreenText.selfPokemonLabel}
            pokemon={pokemon}
            value={selfId}
            onChange={setSelfId}
          />
          {selfPokemon !== undefined && <SpeedPokemonCard pokemon={selfPokemon} />}
          <div className="speed-field">
            <span className="speed-field__label">{speedScreenText.rankLabel}</span>
            <RankMeter
              label={speedScreenText.rankLabel}
              statName={rankStatNameJa.spe}
              value={rank}
              onChange={(value) => {
                setRank(Math.min(MAX_RANK, Math.max(MIN_RANK, value)));
              }}
              asGroup={false}
            />
          </div>
          <div className="speed-chips">
            <ToggleChip
              type="checkbox"
              label={speedScreenText.scarfLabel}
              checked={scarf}
              onChange={setScarf}
            />
            <ToggleChip
              type="checkbox"
              label={speedScreenText.selfTailwindLabel}
              checked={selfTailwind}
              onChange={setSelfTailwind}
            />
            <ToggleChip
              type="checkbox"
              label={speedScreenText.selfParalysisLabel}
              checked={selfParalysis}
              onChange={setSelfParalysis}
            />
            <ToggleChip
              type="checkbox"
              label={speedScreenText.currentToggleLabel}
              checked={useCurrent}
              onChange={setUseCurrent}
            />
          </div>
          <p className="speed-hint">{speedScreenText.currentHint}</p>
          {useCurrent && (
            <div className="speed-current">
              <Stepper
                label={speedScreenText.spLabel}
                value={sp}
                min={0}
                max={MAX_SP_PER_STAT}
                onChange={setSp}
              />
              <SegmentedControl
                label={speedScreenText.natureGroupLabel}
                options={NATURES.map((value) => ({ value, label: speedScreenText.natureLabel[value] }))}
                value={nature}
                onChange={setNature}
              />
            </div>
          )}
        </section>

        <section
          aria-labelledby={opponentHeadingId}
          className="speed-card speed-card--typed"
          style={typeAccentStyle(opponentPokemon?.types[0])}
        >
          <h2 id={opponentHeadingId} className="speed-card__region">
            {speedScreenText.opponentRegionLabel}
          </h2>
          <PokemonSelect
            label={speedScreenText.opponentPokemonLabel}
            pokemon={pokemon}
            value={opponentId}
            onChange={setOpponentId}
          />
          {opponentPokemon !== undefined && <SpeedPokemonCard pokemon={opponentPokemon} />}
          <fieldset className="speed-group">
            <legend className="speed-field__label">{speedScreenText.assumptionGroupLabel}</legend>
            <div className="speed-chips">
              {PRESET_ORDER.map((preset) => (
                <ToggleChip
                  key={preset}
                  type="checkbox"
                  label={speedPresetText[preset]}
                  checked={assumptions.has(preset)}
                  ariaDisabled={assumptions.has(preset) && assumptions.size <= 1}
                  onChange={(checked) => {
                    toggleAssumption(preset, checked);
                  }}
                />
              ))}
            </div>
          </fieldset>
          <p className="speed-hint">{speedScreenText.assumptionHint}</p>
          {minimumNotice && (
            <p className="speed-hint speed-hint--warn">{speedScreenText.assumptionMinimumNotice}</p>
          )}
          <div className="speed-chips">
            <ToggleChip
              type="checkbox"
              label={speedScreenText.opponentTailwindLabel}
              checked={opponentTailwind}
              onChange={setOpponentTailwind}
            />
            <ToggleChip
              type="checkbox"
              label={speedScreenText.opponentParalysisLabel}
              checked={opponentParalysis}
              onChange={setOpponentParalysis}
            />
          </div>
        </section>
      </div>

      <fieldset className="speed-card speed-field-state">
        <legend className="speed-card__region">{speedScreenText.fieldGroupLabel}</legend>
        <div className="speed-chips">
          <ToggleChip
            type="checkbox"
            label={speedScreenText.trickRoomLabel}
            checked={trickRoom}
            onChange={setTrickRoom}
          />
        </div>
        <p className="speed-hint">{speedScreenText.trickRoomHint}</p>
      </fieldset>

      <section aria-labelledby={matchupHeadingId} className="speed-card">
        <h2 id={matchupHeadingId} className="speed-card__heading">
          {matchupHeading}
        </h2>
        {matchupKey === null && <p className="speed-screen__notice">{speedScreenText.introNotice}</p>}
        {matchup.loading && <p className="speed-screen__notice">{speedScreenText.matchupLoadingNotice}</p>}
        {matchup.result?.ok === false && (
          <p role="alert" className="speed-screen__error">
            {errorText(matchup.result.error.code)}
          </p>
        )}
        {matchup.result?.ok === true && (
          <MatchupCards
            response={matchup.result.value}
            trickRoom={trickRoom}
            accentType={opponentPokemon?.types[0]}
          />
        )}
        {onOpenCombo !== undefined && selfId !== "" && opponentId !== "" && (
          <button
            type="button"
            className="ui-button ui-button--secondary speed-combo"
            onClick={() => {
              onOpenCombo({
                attackerPokemonId: selfId,
                defenderPokemonId: opponentId,
                ...(useCurrent ? { speed: { sp, nature, rank, scarf } } : {}),
              });
            }}
          >
            {speedScreenText.comboLinkLabel}
          </button>
        )}
      </section>

      <section aria-labelledby={reachHeadingId} className="speed-card">
        <h2 id={reachHeadingId} className="speed-card__heading">
          {speedScreenText.reachHeading}
        </h2>
        <p className="speed-hint">{speedScreenText.reachIntro}</p>
        {reachKey === null && <p className="speed-screen__notice">{speedScreenText.reachNeedsCurrent}</p>}
        {reach.loading && <p className="speed-screen__notice">{speedScreenText.reachLoadingNotice}</p>}
        {reach.result?.ok === false && (
          <p role="alert" className="speed-screen__error">
            {errorText(reach.result.error.code)}
          </p>
        )}
        {reach.result?.ok === true && <ReachList response={reach.result.value} trickRoom={trickRoom} />}
      </section>
    </div>
  );
}

// ---- 入力の部品 ----

function PokemonSelect({
  label,
  pokemon,
  value,
  onChange,
}: {
  readonly label: string;
  readonly pokemon: readonly SpeedPokemon[];
  readonly value: string;
  readonly onChange: (pokemonId: string) => void;
}): ReactElement {
  return (
    <select
      aria-label={label}
      className="speed-select"
      value={value}
      onChange={(event) => {
        onChange(event.target.value);
      }}
    >
      <option value="">{speedScreenText.unselectedOption}</option>
      {pokemon.map((entry) => (
        <option key={entry.pokemonId} value={entry.pokemonId}>
          {entry.nameJa}
        </option>
      ))}
    </select>
  );
}

function SpeedPokemonCard({ pokemon }: { readonly pokemon: SpeedPokemon }): ReactElement {
  return (
    <PokemonCard
      speciesKey={pokemon.pokemonId}
      name={pokemon.nameJa}
      types={pokemon.types}
      nameAsHeading
      className="speed-card__face"
    />
  );
}

// ---- 出力 1: 想定ごとのカード ----

/** 上を取れる(先に動ける)性格が 1 つでもある想定か。 */
function canMoveFirst(line: Schemas["MatchupLine"]): boolean {
  return line.natures.some((entry) => entry.movesFirst !== undefined);
}

function MatchupCards({
  response,
  trickRoom,
  accentType,
}: {
  readonly response: Schemas["MatchupResponse"];
  readonly trickRoom: boolean;
  readonly accentType: string | undefined;
}): ReactElement {
  const reachable = response.lines.filter(canMoveFirst).length;
  return (
    <>
      <p className="speed-conclusion" data-testid="speed-conclusion">
        {speedScreenText.matchupConclusion(reachable, response.lines.length, trickRoom)}
      </p>
      {response.selfSpeed !== undefined && (
        <p className="speed-self-speed">{speedScreenText.selfSpeedLabel(response.selfSpeed)}</p>
      )}
      <div className="speed-matchup-list" style={typeAccentStyle(accentType)}>
        {response.lines.map((line, index) => (
          <MatchupCard
            key={line.assumption.preset ?? `custom-${String(index)}`}
            line={line}
            trickRoom={trickRoom}
          />
        ))}
      </div>
    </>
  );
}

/** 一言見出し(ADR-0610 §1・§7。補正なしの最小 SP / 上限 SP から導く)。 */
function headlineOf(natures: readonly Schemas["NatureLine"][], trickRoom: boolean): string {
  const neutral = natures.find((entry) => entry.nature === "neutral")?.movesFirst;
  const plus = natures.find((entry) => entry.nature === "plus")?.movesFirst;
  if (trickRoom) {
    return neutral === undefined
      ? speedScreenText.unreachableTrickRoomNotice
      : speedScreenText.headlineTrickRoom(neutral.max);
  }
  if (neutral !== undefined) {
    return neutral.min === 0 ? speedScreenText.headlineUninvested : speedScreenText.headlineNeutralMax;
  }
  return plus === undefined ? speedScreenText.unreachableNotice : speedScreenText.headlinePlusOnly(plus.min);
}

function MatchupCard({
  line,
  trickRoom,
}: {
  readonly line: Schemas["MatchupLine"];
  readonly trickRoom: boolean;
}): ReactElement {
  const preset = line.assumption.preset;
  const heading =
    preset === undefined
      ? speedScreenText.customCardHeading(line.opponentSpeed)
      : speedScreenText.cardHeading(preset, line.opponentSpeed);
  // 下降補正の行はトリックルーム中だけ出す(ADR-0610 §6 Q5)。
  const shown = line.natures.filter((entry) => trickRoom || entry.nature !== "minus");
  return (
    <article className="speed-matchup-card" data-testid="speed-matchup-card" data-preset={preset ?? "custom"}>
      <h3 className="speed-matchup-card__title">{heading}</h3>
      <p className="speed-matchup-card__headline" data-testid="speed-headline">
        {headlineOf(line.natures, trickRoom)}
      </p>
      {line.current !== undefined && (
        <p className={`speed-order speed-order--${line.current}`} data-testid="speed-current-order">
          {speedScreenText.currentOrderLabel[line.current]}
        </p>
      )}
      <details className="speed-details">
        <summary className="speed-details__summary">
          {trickRoom ? speedScreenText.naturesSummaryTrickRoom : speedScreenText.naturesSummary}
        </summary>
        <ul className="speed-natures">
          {shown.map((entry) => (
            <NatureRow key={entry.nature} line={entry} trickRoom={trickRoom} />
          ))}
        </ul>
      </details>
    </article>
  );
}

function NatureRow({
  line,
  trickRoom,
}: {
  readonly line: Schemas["NatureLine"];
  readonly trickRoom: boolean;
}): ReactElement {
  const { movesFirst, tie } = line;
  let main: string;
  if (movesFirst === undefined) {
    main = trickRoom ? speedScreenText.cannotMoveFirstTrickRoom : speedScreenText.cannotMoveFirst;
  } else {
    main = trickRoom
      ? speedScreenText.movesFirstMax(movesFirst.max)
      : speedScreenText.movesFirstMin(movesFirst.min);
  }
  return (
    <li
      className={`speed-nature speed-nature--${line.nature}`}
      data-testid="speed-nature-line"
      data-nature={line.nature}
    >
      <span className="speed-nature__label">{speedScreenText.natureLabel[line.nature]}</span>
      <span className="speed-nature__main">{main}</span>
      {tie !== undefined && (
        <span className="speed-nature__tie">{speedScreenText.tieAt(tie.min, tie.max)}</span>
      )}
    </li>
  );
}

// ---- 出力 2: これ以上 ----

function ReachList({
  response,
  trickRoom,
}: {
  readonly response: Schemas["ReachResponse"];
  readonly trickRoom: boolean;
}): ReactElement {
  const highlight = response.lines.find((line) => line.preset === "max");
  return (
    <>
      {highlight !== undefined && (
        <p className="speed-conclusion" data-testid="speed-reach-conclusion">
          {reachSlowerText(highlight, trickRoom)}
        </p>
      )}
      <details className="speed-details">
        <summary className="speed-details__summary">
          {speedScreenText.reachListSummary(response.lines.length)}
        </summary>
        <ul className="speed-reach">
          {response.lines.map((line) => (
            <ReachRow key={line.preset} line={line} trickRoom={trickRoom} />
          ))}
        </ul>
      </details>
    </>
  );
}

/** 「抜ける側」の文(行の slower セルと結論の 1 行で同じ文を使う)。 */
function reachSlowerText(line: Schemas["ReachLine"], trickRoom: boolean): string {
  const t = speedScreenText;
  const { preset, slower } = line;
  if (slower === undefined) {
    return trickRoom ? t.reachNoneSlowerTrickRoom : t.reachNoneSlower;
  }
  return trickRoom ? t.reachSlowerTrickRoom(preset, slower.max) : t.reachSlower(preset, slower.max);
}

function ReachRow({
  line,
  trickRoom,
}: {
  readonly line: Schemas["ReachLine"];
  readonly trickRoom: boolean;
}): ReactElement {
  const { preset, slower, tie, faster } = line;
  const t = speedScreenText;
  return (
    <li className="speed-reach__row" data-testid="speed-reach-line" data-preset={preset}>
      <h3 className="speed-reach__title">{speedPresetText[preset]}</h3>
      <ReachCell
        testId="speed-reach-slower"
        text={reachSlowerText(line, trickRoom)}
        count={slower === undefined ? undefined : line.slowerCount}
        example={line.fastestSlower}
      />
      {tie !== undefined && (
        <ReachCell
          testId="speed-reach-tie"
          text={t.reachTie(preset, tie.min, tie.max)}
          count={line.tieCount}
          example={undefined}
        />
      )}
      <ReachCell
        testId="speed-reach-faster"
        text={
          faster === undefined
            ? trickRoom
              ? t.reachNoneFasterTrickRoom
              : t.reachNoneFaster
            : trickRoom
              ? t.reachFasterTrickRoom(preset, faster.min)
              : t.reachFaster(preset, faster.min)
        }
        count={faster === undefined ? undefined : line.fasterCount}
        example={line.slowestFaster}
      />
    </li>
  );
}

function ReachCell({
  testId,
  text,
  count,
  example,
}: {
  readonly testId: string;
  readonly text: string;
  readonly count: number | undefined;
  readonly example: SpeedPokemon | undefined;
}): ReactElement {
  return (
    <div className="speed-reach__cell" data-testid={testId}>
      <span className="speed-reach__text">{text}</span>
      {count !== undefined && <span className="speed-reach__count">{speedScreenText.reachCount(count)}</span>}
      {example !== undefined && (
        <span className="speed-reach__example">{speedScreenText.reachExample(example.nameJa)}</span>
      )}
    </div>
  );
}
