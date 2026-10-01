export type Hypothesis = { text: string; stable: string };

// Consecutive overlapping-window hypotheses agree only on complete words.
// Silence resolves the entire last hypothesis; a bounded window resolves even
// unstable tails so recognition cannot accumulate unbounded audio/latency.
export function agree(previous: string, current: string): Hypothesis {
  const before = previous.trim().split(/\s+/);
  const after = current.trim().split(/\s+/);
  let count = 0;
  while (
    count < before.length &&
    count < after.length &&
    before[count] === after[count]
  )
    count++;
  return { text: current.trim(), stable: after.slice(0, count).join(" ") };
}

export function shouldCommit(silenceMs: number, durationMs: number): boolean {
  return silenceMs >= 600 || durationMs >= 12000;
}
