export function formatViewers(n: number | null): string | null {
  if (n === null || n === undefined) return null;
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return n.toLocaleString();
}

export function isPositiveCount(
  count: number | null | undefined,
): count is number {
  return typeof count === "number" && count > 0;
}
