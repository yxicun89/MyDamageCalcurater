import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { App } from "./App";

test("アプリが描画される", () => {
  render(<App />);
  expect(screen.getByRole("main")).toBeInTheDocument();
});
