export function advanceUptime(value: string | undefined, elapsed: number) {
  if (!value) return value;
  const units: Record<string, number> = { h: 3600, m: 60, s: 1 };
  let seconds = Math.max(0, Math.floor(elapsed));
  for (const match of value.matchAll(/(\d+(?:\.\d+)?)(h|m|s)/g))
    seconds += Number(match[1]) * units[match[2]];
  seconds = Math.floor(seconds);
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  return `${hours ? `${hours}h` : ""}${hours || minutes ? `${minutes}m` : ""}${seconds % 60}s`;
}
