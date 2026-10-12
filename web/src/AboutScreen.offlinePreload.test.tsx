// ADR-0358 段 3: 保存の節は offlinePreload が渡されたときだけ出る。オフラインでは押せない。

import { render, screen } from "@testing-library/react";
import { describe, expect, test } from "vitest";
import { AboutScreen } from "./AboutScreen";
import { offlinePreloadText } from "./i18n/offlineScope";
import type { SearchableMasterSource } from "./master/types";

const source: SearchableMasterSource = {
  load: () => new Promise(() => undefined),
  search: {
    searchSpecies: () => Promise.resolve([]),
    resolveSpecies: () => new Promise(() => undefined),
  },
};

function renderAbout(props: { offlinePreload?: boolean; online?: boolean }) {
  render(
    <AboutScreen
      backHref="/calc"
      onBack={() => undefined}
      focusOnMount={false}
      online={props.online}
      offlinePreload={
        props.offlinePreload === true
          ? { source, loadSavedKeys: () => Promise.resolve(new Set<string>()) }
          : undefined
      }
    />,
  );
}

describe("このアプリについての保存の節", () => {
  test("口が無ければ出さない", () => {
    renderAbout({});
    expect(screen.queryByRole("button", { name: offlinePreloadText.startButton })).toBeNull();
  });

  test("オンラインなら押せる", () => {
    renderAbout({ offlinePreload: true, online: true });
    expect(screen.getByRole("button", { name: offlinePreloadText.startButton })).toBeEnabled();
  });

  test("オフラインなら押せない", () => {
    renderAbout({ offlinePreload: true, online: false });
    expect(screen.getByRole("button", { name: offlinePreloadText.startButton })).toBeDisabled();
  });
});
