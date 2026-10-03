import { render, screen, within } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { App } from "../App";
import { installFakeApi, seedSettings, type FakeApi } from "./fakeApi";
import { scenario } from "./scenario";

/** 保存済みの設定で App を描画し、ホームの商品が出るまで待つ。 */
export async function openHome(
  over: Partial<ReturnType<typeof scenario>> = {},
): Promise<{ api: FakeApi; user: ReturnType<typeof userEvent.setup> }> {
  const s = { ...scenario(), ...over };
  const api = installFakeApi(s);
  seedSettings();
  const user = userEvent.setup();
  render(<App />);
  await screen.findByRole("button", { name: s.items[0]?.name ?? "" });
  return { api, user };
}

export const grid = () => screen.getByTestId("item-grid");
export const gridItem = (name: string) => within(grid()).getByRole("button", { name });
export const queryGridItem = (name: string) => within(grid()).queryByRole("button", { name });
