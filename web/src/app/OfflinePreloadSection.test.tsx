// ADR-0358 段 3: 「オフライン用にポケモンのデータを保存」の節(進捗・中断・失敗の再開・フォーカス・a11y)。

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import { offlinePreloadText } from "../i18n/offlineScope";
import { createCachingMasterSource, listSavedSpeciesKeys } from "../master/cache/cachedSources";
import type { MasterData, MasterSpecies, SearchableMasterSource } from "../master/types";
import { createMemoryMasterCacheStore } from "../test/memoryMasterCacheStore";
import { OfflinePreloadSection } from "./OfflinePreloadSection";

const KEYS = Array.from({ length: 25 }, (_unused, index) => `${String(index + 1).padStart(4, "0")}-000`);

function build(options: { gate?: Promise<void>; failAt?: string } = {}) {
  const store = createMemoryMasterCacheStore();
  let failAt = options.failAt;
  const raw: SearchableMasterSource = {
    load: () =>
      Promise.resolve({
        species: [],
        moves: [],
        items: [],
        abilities: [],
        natures: [],
      } as unknown as MasterData),
    search: {
      searchSpecies: (query) =>
        Promise.resolve(
          query === "ア"
            ? KEYS.map((key) => ({ key, dexNo: 1, form: 0, nameJa: `ア${key}`, types: ["normal"] }))
            : [],
        ),
      async resolveSpecies(key, signal) {
        if (key === KEYS[2]) {
          await options.gate;
        }
        if (signal?.aborted === true) {
          throw new DOMException("aborted", "AbortError");
        }
        if (key === failAt) {
          throw new Error("down");
        }
        return { species: { key } as unknown as MasterSpecies, abilities: [], moves: [] };
      },
    },
  };
  const source = createCachingMasterSource({ source: raw, store });
  return {
    store,
    source,
    loadSavedKeys: () => listSavedSpeciesKeys(store),
    recover: () => {
      failAt = undefined;
    },
  };
}

describe("OfflinePreloadSection", () => {
  test("オンライン: 押すと進捗を polite の status に出し、完了で保存済み件数を出す。フォーカスはボタンに戻る", async () => {
    const user = userEvent.setup();
    const deps = build();
    render(<OfflinePreloadSection online preload={deps} />);
    const start = screen.getByRole("button", { name: offlinePreloadText.startButton });
    await user.click(start);
    await waitFor(() => {
      expect(screen.getByRole("status")).toHaveTextContent(offlinePreloadText.completed(25));
    });
    expect(screen.getByRole("status")).toHaveAttribute("aria-live", "polite");
    expect(screen.getByRole("button", { name: offlinePreloadText.startButton })).toHaveFocus();
    expect((await deps.loadSavedKeys()).size).toBe(25);
  });

  test("実行中は中断ボタンにフォーカスが移り、中断で止まって続きから再開できる", async () => {
    const user = userEvent.setup();
    let release: () => void = () => undefined;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    const deps = build({ gate });
    render(<OfflinePreloadSection online preload={deps} />);
    await user.click(screen.getByRole("button", { name: offlinePreloadText.startButton }));
    const stop = await screen.findByRole("button", { name: offlinePreloadText.stopButton });
    expect(stop).toHaveFocus();
    expect(screen.queryByRole("button", { name: offlinePreloadText.startButton })).toBeNull();
    await user.click(stop);
    release();
    await waitFor(() => {
      expect(screen.getByRole("status")).toHaveTextContent(/中断しました。保存済み: 2匹/);
    });
    expect(screen.getByRole("button", { name: offlinePreloadText.resumeButton })).toHaveFocus();
  });

  test("通信が失敗したら alert で件数と再開の案内を出し、再開で完了する", async () => {
    const user = userEvent.setup();
    const deps = build({ failAt: KEYS[4] });
    render(<OfflinePreloadSection online preload={deps} />);
    await user.click(screen.getByRole("button", { name: offlinePreloadText.startButton }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(offlinePreloadText.failedServer(4));
    expect(alert).toHaveTextContent("続きから");
    deps.recover();
    await user.click(screen.getByRole("button", { name: offlinePreloadText.resumeButton }));
    await waitFor(() => {
      expect(screen.getByRole("status")).toHaveTextContent(offlinePreloadText.completed(25));
    });
    expect(screen.queryByRole("alert")).toBeNull();
  });

  test("オフラインでは押せず、理由を出す", () => {
    render(<OfflinePreloadSection online={false} preload={build()} />);
    const button = screen.getByRole("button", { name: offlinePreloadText.startButton });
    expect(button).toBeDisabled();
    expect(button).toHaveAccessibleDescription(offlinePreloadText.offlineReason);
  });

  test("UI の文言に「種族」を使わない", () => {
    render(<OfflinePreloadSection online preload={build()} />);
    expect(document.body.textContent).not.toContain("種族");
  });
});
