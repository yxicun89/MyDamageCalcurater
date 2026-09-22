// SP3: 素早さ比較の画面(ADR-0604 §4・§6、docs/speed-design.md §5・§6)。
// SpeedClient は fake(props で注入)。engine(WASM)・pokedex のマスタは使わない(ADR-0604 §5)。
// 確かめること(受け入れ条件):
//   A1 マウント時に pokemon() と table() を1回ずつ呼ぶ(position はまだ呼ばない)
//   A2 両方そろうまで「左の表だけ」ローディング(右の入力は先に使える。ADR-0604 §4)
//   A3 左は tiers を速い順のまま描画し、2行以上の段には「同速」を出す(Web で並べ替え直さない。ADR-0601 §3)
//   A4 各行にタイプ色のエンブレム(data-testid="type-emblem"、CalcScreen と同じ形)・名前・調整名・実数値を出す
//   A5 右のモード(preset / custom / raw)で入力欄が変わり、入力のたびに position() を呼び直す
//   A6 古い応答は無視する(cancelled フラグ。後から届いた前の入力の結果で表示を上書きしない)
//   A7 tie があれば左の同じ段を強調し、無ければ faster/slower の境界に印を出す(ADR-0604 §4)
//   A8 エラー(SpeedResult.ok=false)でも表示が壊れない(role=alert を出し、他方の表示を消さない)

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import { speedPresetText, speedScreenText } from "../i18n/ja";
import { SpeedScreen } from "./SpeedScreen";
import type { components } from "./speed.gen";
import type { SpeedClient, SpeedResult } from "./speedClient";

type Schemas = components["schemas"];

// ---- fake の SpeedClient(呼び出しを記録し、テストが応答を返す。BalanceScreen.test.tsx と同じ形) ----

interface PendingCall<A, T> {
  readonly args: A;
  resolve(result: SpeedResult<T>): void;
}

interface FakeSpeedClient extends SpeedClient {
  readonly pokemonCalls: PendingCall<null, Schemas["PokemonListResponse"]>[];
  readonly tableCalls: PendingCall<readonly Schemas["PresetId"][] | undefined, Schemas["TableResponse"]>[];
  readonly positionCalls: PendingCall<Schemas["PositionRequest"], Schemas["PositionResponse"]>[];
}

function createFakeSpeedClient(): FakeSpeedClient {
  const pokemonCalls: FakeSpeedClient["pokemonCalls"] = [];
  const tableCalls: FakeSpeedClient["tableCalls"] = [];
  const positionCalls: FakeSpeedClient["positionCalls"] = [];
  return {
    pokemonCalls,
    tableCalls,
    positionCalls,
    pokemon() {
      return new Promise((resolve) => {
        pokemonCalls.push({ args: null, resolve });
      });
    },
    table(presets) {
      return new Promise((resolve) => {
        tableCalls.push({ args: presets === undefined ? undefined : [...presets], resolve });
      });
    },
    position(request) {
      return new Promise((resolve) => {
        positionCalls.push({ args: structuredClone(request), resolve });
      });
    },
  };
}

function lastOf<T>(calls: readonly T[], name: string): T {
  const call = calls.at(-1);
  if (call === undefined) {
    throw new Error(`${name} が呼ばれていない`);
  }
  return call;
}

// ---- 架空データ(services/speed/testdata と同じ 9xxx-xxx の形。実マスタは使わない。ADR-0002) ----

const BIRD: Schemas["SpeedPokemon"] = {
  pokemonId: "9001-000",
  nameJa: "テストカソウドリ",
  types: ["fire", "flying"],
  baseSpeed: 100,
};
const FISH: Schemas["SpeedPokemon"] = {
  pokemonId: "9002-000",
  nameJa: "テストカソウギョ",
  types: ["water"],
  baseSpeed: 80,
};
const GRASS: Schemas["SpeedPokemon"] = {
  pokemonId: "9003-000",
  nameJa: "テストカソウソウ",
  types: ["grass"],
  baseSpeed: 50,
};

const pokemonListResponse: Schemas["PokemonListResponse"] = {
  regulationId: "example",
  pokemon: [BIRD, FISH, GRASS],
};

function entry(pokemon: Schemas["SpeedPokemon"], preset: Schemas["PresetId"]): Schemas["SpeedTableEntry"] {
  return { ...pokemon, preset };
}

/**
 * 段は速い順(300・200・150・100)。200 の段だけ2行(同速)。
 * 行の総数は 1 + 2 + 1 + 1 = 5(境界の計算は「段」ではなく「行」の数で決まる。ADR-0604 §4)。
 */
const tableResponse: Schemas["TableResponse"] = {
  regulationId: "example",
  presets: ["uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"],
  tiers: [
    { speed: 300, entries: [entry(BIRD, "max-scarf")] },
    { speed: 200, entries: [entry(BIRD, "max"), entry(FISH, "max-scarf")] },
    { speed: 150, entries: [entry(FISH, "max")] },
    { speed: 100, entries: [entry(GRASS, "uninvested")] },
  ],
};

// ---- 描画と、よく使う問い合わせ ----

function renderScreen(): { user: UserEvent; client: FakeSpeedClient } {
  const user = userEvent.setup();
  const client = createFakeSpeedClient();
  render(<SpeedScreen speedClient={client} />);
  return { user, client };
}

/** 左(速い順の表)の領域。 */
function tableRegion(): HTMLElement {
  return screen.getByRole("region", { name: speedScreenText.tableRegionLabel });
}

/** 右(自分のポケモン)の領域。 */
function selfRegion(): HTMLElement {
  return screen.getByRole("region", { name: speedScreenText.selfRegionLabel });
}

/** 左の表の段(速い順のまま)。段は data-testid="speed-tier"、実数値は data-speed で確かめる。 */
function tiers(): HTMLElement[] {
  return within(tableRegion()).getAllByTestId("speed-tier");
}

/** 1回だけ解決を流す(BalanceScreen.test.tsx と同じ形)。 */
async function flush(resolve: () => void): Promise<void> {
  await act(async () => {
    resolve();
    await Promise.resolve();
  });
}

/** pokemon() と table() の両方を成功で解決する(左の表が出た状態にする)。 */
async function resolveInitial(
  client: FakeSpeedClient,
  table: Schemas["TableResponse"] = tableResponse,
): Promise<void> {
  const pokemonCall = lastOf(client.pokemonCalls, "pokemon");
  const tableCall = lastOf(client.tableCalls, "table");
  await flush(() => {
    pokemonCall.resolve({ ok: true, value: pokemonListResponse });
  });
  await flush(() => {
    tableCall.resolve({ ok: true, value: table });
  });
}

// ---- A1・A2: マウント時の呼び出しと、左だけのローディング ----

describe("A1/A2 マウント時の呼び出し", () => {
  test("pokemon() と table() を1回ずつ呼ぶ(table は presets を省く = 全6行)", () => {
    const { client } = renderScreen();
    expect(client.pokemonCalls).toHaveLength(1);
    expect(client.tableCalls).toHaveLength(1);
    expect(lastOf(client.tableCalls, "table").args).toBeUndefined();
  });

  test("ポケモンを選ぶまで position() は呼ばない", () => {
    const { client } = renderScreen();
    expect(client.positionCalls).toHaveLength(0);
  });

  test("両方そろうまで左の表はローディングのまま(右の入力は先に使える)", async () => {
    const { client } = renderScreen();
    expect(within(tableRegion()).getByText(speedScreenText.loadingNotice)).toBeInTheDocument();

    // pokemon() だけ届いた段階: 右のポケモンの選択肢は出るが、左はまだローディング。
    await flush(() => {
      lastOf(client.pokemonCalls, "pokemon").resolve({ ok: true, value: pokemonListResponse });
    });
    expect(
      within(selfRegion()).getByRole("option", { name: BIRD.nameJa, selected: false }),
    ).toBeInTheDocument();
    expect(within(tableRegion()).getByText(speedScreenText.loadingNotice)).toBeInTheDocument();
    expect(within(tableRegion()).queryAllByTestId("speed-tier")).toHaveLength(0);

    await flush(() => {
      lastOf(client.tableCalls, "table").resolve({ ok: true, value: tableResponse });
    });
    expect(within(tableRegion()).queryByText(speedScreenText.loadingNotice)).not.toBeInTheDocument();
    expect(tiers()).toHaveLength(tableResponse.tiers.length);
  });
});

// ---- 左の表の絞り込み(道具・ランク。ユーザー確定仕様。docs/plan.md「SP: 素早さ比較」) ----

describe("表の絞り込み", () => {
  test("チェックを外すと、外した行を除いた presets で table() を呼び直す", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);
    expect(client.tableCalls).toHaveLength(1);

    const filter = within(tableRegion()).getByRole("group", { name: speedScreenText.filterGroupLabel });
    await user.click(within(filter).getByRole("checkbox", { name: speedPresetText["max-scarf"] }));

    expect(client.tableCalls).toHaveLength(2);
    expect(lastOf(client.tableCalls, "table").args).toEqual([
      "uninvested",
      "neutral-max",
      "max",
      "max-plus1",
      "max-plus2",
    ]);
  });

  test("全部選び直すと presets を省いて呼ぶ(全6行)", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);
    const filter = within(tableRegion()).getByRole("group", { name: speedScreenText.filterGroupLabel });
    const scarfCheckbox = within(filter).getByRole("checkbox", { name: speedPresetText["max-scarf"] });

    await user.click(scarfCheckbox);
    await flush(() => {
      lastOf(client.tableCalls, "table").resolve({ ok: true, value: tableResponse });
    });
    await user.click(scarfCheckbox);

    expect(lastOf(client.tableCalls, "table").args).toBeUndefined();
  });

  test("最後の1つは外せない(契約上 presets は1つ以上)", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);
    const filter = within(tableRegion()).getByRole("group", { name: speedScreenText.filterGroupLabel });

    for (const id of ["neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"] as const) {
      await user.click(within(filter).getByRole("checkbox", { name: speedPresetText[id] }));
      await flush(() => {
        lastOf(client.tableCalls, "table").resolve({ ok: true, value: tableResponse });
      });
    }
    const callsBeforeLast = client.tableCalls.length;
    const lastCheckbox = within(filter).getByRole("checkbox", { name: speedPresetText.uninvested });

    expect(lastCheckbox).toBeDisabled();
    expect(within(filter).getByText(speedScreenText.filterMinimumNotice)).toBeInTheDocument();
    await user.click(lastCheckbox);
    expect(client.tableCalls).toHaveLength(callsBeforeLast);
  });
});

// ---- A3・A4: 左の表 ----

describe("A3/A4 左の表(段・同速・タイプ色のエンブレム)", () => {
  test("段を応答の順(速い順)のまま描画する", async () => {
    const { client } = renderScreen();
    await resolveInitial(client);
    expect(tiers().map((tier) => tier.getAttribute("data-speed"))).toEqual(["300", "200", "150", "100"]);
  });

  test("段には実数値を出す", async () => {
    const { client } = renderScreen();
    await resolveInitial(client);
    const [first] = tiers();
    expect(first).toBeDefined();
    expect(first).toHaveTextContent(speedScreenText.tierSpeedLabel(300));
  });

  test("2行以上の段には「同速」を出し、1行だけの段には出さない", async () => {
    const { client } = renderScreen();
    await resolveInitial(client);
    const [fastest, tied] = tiers();
    expect(fastest).toBeDefined();
    expect(tied).toBeDefined();
    expect(within(tied as HTMLElement).getByText(speedScreenText.tieLabel)).toBeInTheDocument();
    expect(within(fastest as HTMLElement).queryByText(speedScreenText.tieLabel)).not.toBeInTheDocument();
  });

  test("段の中の行は応答の順のまま、名前と調整の表示名を出す(pokemonId 昇順・プリセット順を並べ替え直さない)", async () => {
    const { client } = renderScreen();
    await resolveInitial(client);
    const [, tied] = tiers();
    expect(tied).toBeDefined();
    const entries = within(tied as HTMLElement).getAllByTestId("speed-entry");
    expect(entries).toHaveLength(2);
    expect(entries[0]).toHaveTextContent(BIRD.nameJa);
    expect(entries[0]).toHaveTextContent(speedPresetText.max);
    expect(entries[1]).toHaveTextContent(FISH.nameJa);
    expect(entries[1]).toHaveTextContent(speedPresetText["max-scarf"]);
  });

  test("各行にタイプ色のエンブレム(1つ目のタイプの色)が出る(画像は必須にしない)", async () => {
    const { client } = renderScreen();
    await resolveInitial(client);
    const [, tied] = tiers();
    expect(tied).toBeDefined();
    const emblems = within(tied as HTMLElement).getAllByTestId("type-emblem");
    expect(emblems).toHaveLength(2);
    expect(emblems[0]?.getAttribute("style") ?? "").toContain(`var(--type-${BIRD.types[0] ?? ""})`);
    expect(emblems[1]?.getAttribute("style") ?? "").toContain(`var(--type-${FISH.types[0] ?? ""})`);
  });
});

// ---- A5: 右のモードと position() の呼び直し ----

describe("A5 右のモード切り替えと position()", () => {
  /** モードのラジオ(セグメントコントロール)を選ぶ。 */
  async function selectMode(user: UserEvent, label: string): Promise<void> {
    await user.click(within(selfRegion()).getByRole("radio", { name: label }));
  }

  test("既定は preset(ポケモン・調整・スカーフの入力が出る)", async () => {
    const { client } = renderScreen();
    await resolveInitial(client);
    const region = selfRegion();
    expect(within(region).getByRole("radio", { name: speedScreenText.modeLabel.preset })).toBeChecked();
    expect(within(region).getByRole("combobox", { name: speedScreenText.pokemonLabel })).toBeInTheDocument();
    expect(
      within(region).getByRole("radiogroup", { name: speedScreenText.presetGroupLabel }),
    ).toBeInTheDocument();
    expect(within(region).getByRole("checkbox", { name: speedScreenText.scarfLabel })).toBeInTheDocument();
    // custom / raw だけの入力は出さない(契約上、mode に要らない項目は 400 invalid_request)。
    expect(within(region).queryByRole("spinbutton", { name: speedScreenText.spLabel })).toBeNull();
    expect(within(region).queryByRole("spinbutton", { name: speedScreenText.rawValueLabel })).toBeNull();
  });

  test("preset: ポケモンを選ぶと position() を呼ぶ(mode に要る項目だけを送る)", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);

    await user.selectOptions(
      within(selfRegion()).getByRole("combobox", { name: speedScreenText.pokemonLabel }),
      BIRD.pokemonId,
    );

    await waitFor(() => {
      expect(client.positionCalls.length).toBeGreaterThan(0);
    });
    const expected: Schemas["PositionRequest"] = {
      mode: "preset",
      pokemonId: BIRD.pokemonId,
      preset: "max",
      scarf: false,
    };
    expect(lastOf(client.positionCalls, "position").args).toEqual(expected);
  });

  test("preset: 調整・スカーフを変えるたびに position() を呼び直す", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);
    const region = selfRegion();
    await user.selectOptions(
      within(region).getByRole("combobox", { name: speedScreenText.pokemonLabel }),
      BIRD.pokemonId,
    );
    await waitFor(() => {
      expect(client.positionCalls.length).toBeGreaterThan(0);
    });
    const afterSelect = client.positionCalls.length;

    await user.click(within(region).getByRole("radio", { name: speedPresetText.uninvested }));
    await waitFor(() => {
      expect(client.positionCalls.length).toBeGreaterThan(afterSelect);
    });
    expect(lastOf(client.positionCalls, "position").args).toEqual({
      mode: "preset",
      pokemonId: BIRD.pokemonId,
      preset: "uninvested",
      scarf: false,
    } satisfies Schemas["PositionRequest"]);

    await user.click(within(region).getByRole("checkbox", { name: speedScreenText.scarfLabel }));
    await waitFor(() => {
      expect(lastOf(client.positionCalls, "position").args).toEqual({
        mode: "preset",
        pokemonId: BIRD.pokemonId,
        preset: "uninvested",
        scarf: true,
      } satisfies Schemas["PositionRequest"]);
    });
  });

  test("custom: SP・性格・ランク・スカーフの入力に変わり、その形で position() を呼ぶ", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);
    await selectMode(user, speedScreenText.modeLabel.custom);
    const region = selfRegion();

    expect(within(region).getByRole("spinbutton", { name: speedScreenText.spLabel })).toBeInTheDocument();
    expect(
      within(region).getByRole("radiogroup", { name: speedScreenText.natureGroupLabel }),
    ).toBeInTheDocument();
    expect(within(region).getByRole("spinbutton", { name: speedScreenText.rankLabel })).toBeInTheDocument();
    // preset だけの入力は出さない。
    expect(within(region).queryByRole("radiogroup", { name: speedScreenText.presetGroupLabel })).toBeNull();

    await user.selectOptions(
      within(region).getByRole("combobox", { name: speedScreenText.pokemonLabel }),
      BIRD.pokemonId,
    );
    await user.click(within(region).getByRole("radio", { name: speedScreenText.natureLabel.plus }));

    await waitFor(() => {
      const { args } = lastOf(client.positionCalls, "position");
      expect(args.mode).toBe("custom");
      expect(args.pokemonId).toBe(BIRD.pokemonId);
      expect(args.nature).toBe("plus");
      expect(typeof args.sp).toBe("number");
      expect(typeof args.rank).toBe("number");
      expect(args.scarf).toBe(false);
      // custom では preset / value を送らない(契約: mode に要らない項目は 400)。
      expect(args.preset).toBeUndefined();
      expect(args.value).toBeUndefined();
    });
  });

  test("raw: 実数値の入力に変わり、{mode: raw, value} だけを送る(ポケモンは任意)", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);
    await selectMode(user, speedScreenText.modeLabel.raw);
    const region = selfRegion();

    const value = within(region).getByRole("spinbutton", { name: speedScreenText.rawValueLabel });
    // 実数値を入れるまでは呼ばない。
    expect(client.positionCalls).toHaveLength(0);

    await user.type(value, "180");

    await waitFor(() => {
      expect(lastOf(client.positionCalls, "position").args).toEqual({
        mode: "raw",
        value: 180,
      } satisfies Schemas["PositionRequest"]);
    });
  });

  test("結果(実数値・速い/遅い件数・同速)を応答のまま出す", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);
    await user.selectOptions(
      within(selfRegion()).getByRole("combobox", { name: speedScreenText.pokemonLabel }),
      BIRD.pokemonId,
    );
    await waitFor(() => {
      expect(client.positionCalls.length).toBeGreaterThan(0);
    });

    await flush(() => {
      lastOf(client.positionCalls, "position").resolve({
        ok: true,
        value: { speed: 200, faster: 1, slower: 2, tie: [entry(FISH, "max-scarf")], pokemon: BIRD },
      });
    });

    const region = selfRegion();
    expect(region).toHaveTextContent(speedScreenText.selfSpeedLabel(200));
    expect(region).toHaveTextContent(speedScreenText.fasterLabel(1));
    expect(region).toHaveTextContent(speedScreenText.slowerLabel(2));
    expect(region).toHaveTextContent(FISH.nameJa);
  });
});

// ---- A6: 古い応答の無視(cancelled フラグ) ----

describe("A6 古い応答の無視", () => {
  test("入力を変えた後に届いた前の応答で表示を上書きしない", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);
    const pokemonSelect = within(selfRegion()).getByRole("combobox", {
      name: speedScreenText.pokemonLabel,
    });

    await user.selectOptions(pokemonSelect, BIRD.pokemonId);
    await waitFor(() => {
      expect(client.positionCalls.length).toBeGreaterThan(0);
    });
    const staleCall = lastOf(client.positionCalls, "position");

    await user.selectOptions(pokemonSelect, FISH.pokemonId);
    await waitFor(() => {
      expect(lastOf(client.positionCalls, "position")).not.toBe(staleCall);
    });
    const freshCall = lastOf(client.positionCalls, "position");

    await flush(() => {
      freshCall.resolve({ ok: true, value: { speed: 150, faster: 3, slower: 1, tie: [], pokemon: FISH } });
    });
    await flush(() => {
      staleCall.resolve({ ok: true, value: { speed: 300, faster: 0, slower: 4, tie: [], pokemon: BIRD } });
    });

    const region = selfRegion();
    expect(region).toHaveTextContent(speedScreenText.selfSpeedLabel(150));
    expect(region).not.toHaveTextContent(speedScreenText.selfSpeedLabel(300));
  });

  test("入力を変えたら、新しい応答が届くまで前の結果を出したままにしない", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);
    const pokemonSelect = within(selfRegion()).getByRole("combobox", {
      name: speedScreenText.pokemonLabel,
    });

    await user.selectOptions(pokemonSelect, BIRD.pokemonId);
    await waitFor(() => {
      expect(client.positionCalls.length).toBeGreaterThan(0);
    });
    await flush(() => {
      lastOf(client.positionCalls, "position").resolve({
        ok: true,
        value: { speed: 300, faster: 0, slower: 4, tie: [], pokemon: BIRD },
      });
    });
    expect(selfRegion()).toHaveTextContent(speedScreenText.selfSpeedLabel(300));

    await user.selectOptions(pokemonSelect, FISH.pokemonId);
    await waitFor(() => {
      expect(selfRegion()).not.toHaveTextContent(speedScreenText.selfSpeedLabel(300));
    });
    expect(within(selfRegion()).getByText(speedScreenText.positionLoadingNotice)).toBeInTheDocument();
  });
});

// ---- A7: 左右の連動(強調・境界線) ----

describe("A7 自分の位置の表示(強調・境界線)", () => {
  /** ポケモンを1体選び、position() の応答を返す。 */
  async function positionWith(
    user: UserEvent,
    client: FakeSpeedClient,
    value: Schemas["PositionResponse"],
  ): Promise<void> {
    await user.selectOptions(
      within(selfRegion()).getByRole("combobox", { name: speedScreenText.pokemonLabel }),
      BIRD.pokemonId,
    );
    await waitFor(() => {
      expect(client.positionCalls.length).toBeGreaterThan(0);
    });
    await flush(() => {
      lastOf(client.positionCalls, "position").resolve({ ok: true, value });
    });
  }

  test("tie があれば、同じ実数値の段だけを強調する(境界線は出さない)", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);

    await positionWith(user, client, {
      speed: 200,
      faster: 1,
      slower: 2,
      tie: [entry(BIRD, "max"), entry(FISH, "max-scarf")],
      pokemon: BIRD,
    });

    expect(tiers().map((tier) => tier.getAttribute("data-self"))).toEqual([null, "tie", null, null]);
    expect(within(tableRegion()).queryByTestId("speed-boundary")).toBeNull();
  });

  test("tie が無ければ、faster 行目の段の直後に境界の印を出す(段ではなく行の数で決める)", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);

    // 実数値 180 は、300(1行)・200(2行)より遅く、150・100 より速い → faster = 3、slower = 2。
    await positionWith(user, client, { speed: 180, faster: 3, slower: 2, tie: [], pokemon: BIRD });

    const boundary = within(tableRegion()).getByTestId("speed-boundary");
    // 直前の段(速い側)は 200、直後の段(遅い側)は 150。
    expect(boundary).toHaveAttribute("data-after-speed", "200");
    expect(boundary).toHaveAttribute("data-before-speed", "150");
    expect(tiers().map((tier) => tier.getAttribute("data-self"))).toEqual([null, null, null, null]);
  });

  test("どの段よりも速いときは、表の先頭に境界の印を出す", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);

    await positionWith(user, client, { speed: 400, faster: 0, slower: 5, tie: [], pokemon: BIRD });

    const boundary = within(tableRegion()).getByTestId("speed-boundary");
    expect(boundary).not.toHaveAttribute("data-after-speed");
    expect(boundary).toHaveAttribute("data-before-speed", "300");
  });

  test("どの段よりも遅いときは、表の末尾に境界の印を出す", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);

    await positionWith(user, client, { speed: 50, faster: 5, slower: 0, tie: [], pokemon: BIRD });

    const boundary = within(tableRegion()).getByTestId("speed-boundary");
    expect(boundary).toHaveAttribute("data-after-speed", "100");
    expect(boundary).not.toHaveAttribute("data-before-speed");
  });

  test("position の結果が届くまでは、強調も境界線も出さない", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);
    await user.selectOptions(
      within(selfRegion()).getByRole("combobox", { name: speedScreenText.pokemonLabel }),
      BIRD.pokemonId,
    );
    await waitFor(() => {
      expect(client.positionCalls.length).toBeGreaterThan(0);
    });

    expect(within(tableRegion()).queryByTestId("speed-boundary")).toBeNull();
    expect(tiers().every((tier) => tier.getAttribute("data-self") === null)).toBe(true);
  });
});

// ---- A8: エラー ----

describe("A8 エラーでも表示が壊れない", () => {
  test("table() が失敗したら左に role=alert を出し、右の入力は使えるままにする", async () => {
    const { client } = renderScreen();
    await flush(() => {
      lastOf(client.pokemonCalls, "pokemon").resolve({ ok: true, value: pokemonListResponse });
    });
    await flush(() => {
      lastOf(client.tableCalls, "table").resolve({
        ok: false,
        error: { code: "master_unavailable", message: "pokemon read model is not configured" },
      });
    });

    expect(within(tableRegion()).getByRole("alert")).toHaveTextContent(
      "pokemon read model is not configured",
    );
    expect(within(tableRegion()).queryAllByTestId("speed-tier")).toHaveLength(0);
    expect(
      within(selfRegion()).getByRole("radiogroup", { name: speedScreenText.modeGroupLabel }),
    ).toBeInTheDocument();
  });

  test("pokemon() が失敗しても、表(table)は出せる", async () => {
    const { client } = renderScreen();
    await flush(() => {
      lastOf(client.pokemonCalls, "pokemon").resolve({
        ok: false,
        error: { code: "speed_unavailable", message: "素早さの API に接続できません" },
      });
    });
    await flush(() => {
      lastOf(client.tableCalls, "table").resolve({ ok: true, value: tableResponse });
    });

    expect(tiers()).toHaveLength(tableResponse.tiers.length);
    expect(screen.getByRole("alert")).toHaveTextContent("素早さの API に接続できません");
  });

  test("position() が失敗しても、左の表と入力は残る", async () => {
    const { user, client } = renderScreen();
    await resolveInitial(client);
    await user.selectOptions(
      within(selfRegion()).getByRole("combobox", { name: speedScreenText.pokemonLabel }),
      BIRD.pokemonId,
    );
    await waitFor(() => {
      expect(client.positionCalls.length).toBeGreaterThan(0);
    });

    await flush(() => {
      lastOf(client.positionCalls, "position").resolve({
        ok: false,
        error: { code: "unknown_pokemon", message: "unknown pokemonId: 9001-000" },
      });
    });

    expect(within(selfRegion()).getByRole("alert")).toHaveTextContent("unknown pokemonId: 9001-000");
    expect(tiers()).toHaveLength(tableResponse.tiers.length);
    expect(within(tableRegion()).queryByTestId("speed-boundary")).toBeNull();
    expect(
      within(selfRegion()).getByRole("combobox", { name: speedScreenText.pokemonLabel }),
    ).toBeInTheDocument();
  });
});
