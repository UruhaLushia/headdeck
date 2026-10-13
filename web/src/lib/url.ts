export function appURL(path: string): string {
  return new URL(path.replace(/^\/+/, ""), document.baseURI).toString();
}
