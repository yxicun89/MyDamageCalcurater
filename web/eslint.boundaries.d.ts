import type { Linter } from "eslint";

export interface KnownViolation {
  wave: string;
  file: string;
  allow: string[];
}
export const KNOWN_VIOLATIONS: readonly KnownViolation[];
export const importBoundaryConfigs: Linter.Config[];
export function buildImportBoundaryConfigs(knownViolations: readonly KnownViolation[]): Linter.Config[];
