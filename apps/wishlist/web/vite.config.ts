import { defineConfig, type UserConfig } from "vite";
import react from "@vitejs/plugin-react";

// クラスタでは前置 /wishlist の下で公開する(docs/design.md W-03)。
export const baseConfig: UserConfig = {
  plugins: [react()],
  base: "/wishlist/",
};

export default defineConfig(baseConfig);
