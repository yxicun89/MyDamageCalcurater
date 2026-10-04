import { join } from "node:path";

/** web/ 直下からの絶対パス(npm script は web/ で実行する前提。jsdom 環境では import.meta.url が file: にならない)。 */
export const webPath = (...p: string[]): string => join(process.cwd(), ...p);
