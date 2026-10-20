import type { Linter } from "eslint";

export interface KnownViolation {
  wave: string;
  file: string;
  allow: string[];
}
export const KNOWN_VIOLATIONS: KnownViolation[];
export const importBoundaryConfigs: Linter.Config[];
