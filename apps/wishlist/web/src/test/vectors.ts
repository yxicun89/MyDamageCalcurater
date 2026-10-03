import vectors from "../../../testdata/query-cases.json";

export interface BuildCase {
  note: string;
  template: string;
  name: string;
  option: string;
  query_override: string | null;
  site_query: string | null;
  want: string;
}
export interface DeeplinkCase {
  note?: string;
  template: string;
  query: string;
  want: string;
}

export const buildCases: BuildCase[] = vectors.build;
export const deeplinkCases: DeeplinkCase[] = vectors.deeplink;
