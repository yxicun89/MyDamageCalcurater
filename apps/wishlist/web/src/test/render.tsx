import { render, screen, within } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { App } from "../App";
import { installFakeApi, seedSettings, type FakeApi } from "./fakeApi";
import { scenario } from "./scenario";

/** App を描画し(5-2 の App は設定が無くてもホームが出る。AC-5-2a-1)、ホームの商品が出るまで待つ。 */
export async function openHome(
  over: Partial<ReturnType<typeof scenario>> = {},
): Promise<{ api: FakeApi; user: ReturnType<typeof userEvent.setup> }> {
  const s = { ...scenario(), ...over };
  const api = installFakeApi(s);
  seedSettings(); // 移行用。fakeApi.ts の seedSettings を参照
  const user = userEvent.setup();
  render(<App />);
  await screen.findByRole("button", { name: s.items[0]?.name ?? "" });
  return { api, user };
}

export const grid = () => screen.getByTestId("item-grid");
export const gridItem = (name: string) => within(grid()).getByRole("button", { name });
export const queryGridItem = (name: string) => within(grid()).queryByRole("button", { name });
