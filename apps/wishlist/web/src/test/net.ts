export function urlOf(input: RequestInfo | URL): string {
  if (typeof input === "string") return input;
  return input instanceof URL ? input.href : input.url;
}
export function bodyText(init: RequestInit): string {
  return typeof init.body === "string" ? init.body : "";
}
