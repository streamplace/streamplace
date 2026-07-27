/**
 * Client-side ABR ("auto quality") for WebRTC playback.
 *
 * WebRTC media is paced, so unlike HLS we can't measure spare bandwidth from
 * bursty downloads — received bitrate ≈ min(rendition bitrate, available
 * bandwidth). The strategy is therefore:
 *
 *  - down fast when the current rendition demonstrably isn't getting through
 *    (delivered bitrate well under nominal, or sustained packet loss),
 *  - up slowly by probing one rung at a time and reverting quickly if the
 *    probe goes bad,
 *  - measure the source rendition's bitrate empirically while it plays clean,
 *    since the server doesn't tell us.
 *
 * This module is deliberately free of React/imports so the heuristics can be
 * read and tuned in one place. All mutable state lives in AbrState; the
 * functions mutate it in place but do no I/O.
 */

export interface AbrRenditionInfo {
  name: string;
  /** Nominal bitrate in bps. Absent for "source" (measured at runtime). */
  bitrate?: number;
}

/** Cumulative counters from a video inbound-rtp stats report. */
export interface AbrSample {
  /** Date.now() at collection time. */
  at: number;
  bytesReceived: number;
  packetsReceived: number;
  packetsLost: number;
}

export interface AbrState {
  /** Rendition the current connection is playing. */
  current: string;
  /** Last time any switch decision was made. */
  lastSwitchAt: number;
  /** Last time an up-probe was started (gates probe frequency). */
  lastProbeAt: number;
  /** Rendition we probed up from, null when not probing. */
  probeFrom: string | null;
  /** When the current up-probe started, null when not probing. */
  probeAt: number | null;
  /** Up-probes suppressed until this time (set after a failed probe). */
  upSuppressedUntil: number;
  /** All switching suppressed until this time (set after a failed/rejected
   *  in-session switch — the switch channel is presumed broken). */
  switchSuppressedUntil: number;
  /** EMA of delivered bitrate (bps) across recent samples. */
  deliveredEma: number | null;
  /** Empirically measured source bitrate (bps), learned while source plays clean. */
  measuredSourceBitrate: number | null;
  /** Recent raw samples for window aggregates. */
  samples: AbrSample[];
}

/** How far back window aggregates look. */
const ABR_WINDOW_MS = 3000;
/** Minimum time between any two switches. */
const MIN_SWITCH_INTERVAL_MS = 3000;
/** Minimum time between up-probes. */
const MIN_PROBE_INTERVAL_MS = 30_000;
/** After an up-probe, this long to decide it failed. */
const PROBE_EVAL_MS = 10_000;
/** Up-probes suppressed for this long after a failed probe. */
const PROBE_FAIL_SUPPRESS_MS = 5 * 60_000;
/** Delivered/nominal below this means the rendition isn't getting through. */
const DOWN_DELIVERY_RATIO = 0.7;
/** Delivered/nominal above this (with low loss) means the connection is clean. */
const CLEAN_DELIVERY_RATIO = 0.9;
/** Packet loss rate that triggers a down-switch. */
const DOWN_LOSS_RATE = 0.03;
/** Packet loss rate under which the connection counts as clean. */
const CLEAN_LOSS_RATE = 0.01;
/** Headroom factor when fitting a tier to measured throughput. */
const FIT_HEADROOM = 0.8;
/** EMA weight for delivered bitrate. */
const EMA_ALPHA = 0.3;

export function createAbrState(current: string, now: number): AbrState {
  return {
    current,
    lastSwitchAt: now,
    lastProbeAt: now,
    probeFrom: null,
    probeAt: null,
    upSuppressedUntil: 0,
    switchSuppressedUntil: 0,
    deliveredEma: null,
    measuredSourceBitrate: null,
    samples: [],
  };
}

/**
 * Feed one stats sample. Counters are per-peer-connection, so a counter reset
 * (reconnect / rendition switch) clears the sample window.
 */
export function addAbrSample(state: AbrState, sample: AbrSample): void {
  const last = state.samples[state.samples.length - 1];
  if (last && sample.bytesReceived < last.bytesReceived) {
    // New peer connection: counters restarted. Keep the EMAs — they describe
    // the network, not the connection — but restart the window.
    state.samples = [];
  } else if (last) {
    const dt = (sample.at - last.at) / 1000;
    if (dt > 0) {
      const inst = (8 * (sample.bytesReceived - last.bytesReceived)) / dt;
      if (inst >= 0 && isFinite(inst)) {
        state.deliveredEma =
          state.deliveredEma === null
            ? inst
            : (1 - EMA_ALPHA) * state.deliveredEma + EMA_ALPHA * inst;
      }
    }
  }
  state.samples.push(sample);
  const cutoff = sample.at - ABR_WINDOW_MS;
  while (state.samples.length > 1 && state.samples[0].at < cutoff) {
    state.samples.shift();
  }
}

interface WindowStats {
  /** Delivered bitrate (bps) across the window. */
  deliveredBps: number;
  /** Packet loss rate across the window, null when too few packets to trust. */
  lossRate: number | null;
  /** Seconds the window spans. */
  span: number;
}

function windowStats(state: AbrState): WindowStats | null {
  const samples = state.samples;
  if (samples.length < 2) return null;
  const first = samples[0];
  const last = samples[samples.length - 1];
  const span = (last.at - first.at) / 1000;
  if (span < 1) return null;
  const deliveredBps = (8 * (last.bytesReceived - first.bytesReceived)) / span;
  const received = last.packetsReceived - first.packetsReceived;
  const lost = Math.max(0, last.packetsLost - first.packetsLost);
  const lossRate = received + lost >= 20 ? lost / (received + lost) : null;
  return { deliveredBps, lossRate, span };
}

/** Video rungs with known bitrates, ascending. "audio" never participates. */
function sortedRungs(ladder: AbrRenditionInfo[]): AbrRenditionInfo[] {
  return ladder
    .filter((r) => r.name !== "audio" && r.name !== "source" && r.bitrate)
    .sort((a, b) => a.bitrate! - b.bitrate!);
}

/** Highest rung that fits in maxBps, or the lowest rung if none do. */
function fitRung(
  rungs: AbrRenditionInfo[],
  maxBps: number,
): AbrRenditionInfo | null {
  if (rungs.length === 0) return null;
  let best = rungs[0];
  for (const r of rungs) {
    if (r.bitrate! <= maxBps) best = r;
    else break;
  }
  return best;
}

/** The rendition above current: next rung up, or "source" above the top rung. */
function rungAbove(rungs: AbrRenditionInfo[], current: string): string | null {
  if (current === "source") return null;
  const idx = rungs.findIndex((r) => r.name === current);
  if (idx === -1) return null;
  return idx + 1 < rungs.length ? rungs[idx + 1].name : "source";
}

/**
 * Re-align the controller with the rendition actually being played. Used when
 * a requested switch was rejected or timed out (pass suppressMs to stop
 * further switch attempts for a while), or when the connection re-established
 * at a different rendition than the controller expects.
 */
export function syncAbrState(
  state: AbrState,
  current: string,
  now: number,
  suppressMs?: number,
): void {
  state.current = current;
  state.probeAt = null;
  state.probeFrom = null;
  state.lastSwitchAt = now;
  if (suppressMs) {
    state.switchSuppressedUntil = now + suppressMs;
  }
}

/**
 * Evaluate current conditions and return the rendition to switch to, or null
 * to stay put. On a non-null return the state is updated (current, timestamps,
 * probe bookkeeping) — the caller is expected to act on the switch, and to
 * call syncAbrState if acting on it fails.
 */
export function decideRendition(
  state: AbrState,
  ladder: AbrRenditionInfo[],
  now: number,
): string | null {
  const rungs = sortedRungs(ladder);
  if (rungs.length === 0) return null;
  if (state.current === "audio") return null;
  if (now < state.switchSuppressedUntil) return null;
  if (now - state.lastSwitchAt < MIN_SWITCH_INTERVAL_MS) return null;

  const win = windowStats(state);
  if (!win) return null;
  const lossy = win.lossRate !== null && win.lossRate > DOWN_LOSS_RATE;
  const clean = win.lossRate !== null && win.lossRate < CLEAN_LOSS_RATE;

  // While source plays clean, learn its bitrate — the server doesn't tell us.
  if (state.current === "source" && clean && state.deliveredEma !== null) {
    state.measuredSourceBitrate =
      state.measuredSourceBitrate === null
        ? state.deliveredEma
        : 0.9 * state.measuredSourceBitrate + 0.1 * state.deliveredEma;
  }

  const currentNominal =
    state.current === "source"
      ? state.measuredSourceBitrate
      : (rungs.find((r) => r.name === state.current)?.bitrate ?? null);

  const starving =
    currentNominal !== null &&
    win.deliveredBps < DOWN_DELIVERY_RATIO * currentNominal;

  // 1) A recent up-probe is failing: revert to where we came from and stop
  //    probing for a while.
  if (
    state.probeAt !== null &&
    now - state.probeAt <= PROBE_EVAL_MS &&
    (lossy || starving)
  ) {
    const target = state.probeFrom ?? rungs[0].name;
    state.probeAt = null;
    state.probeFrom = null;
    state.upSuppressedUntil = now + PROBE_FAIL_SUPPRESS_MS;
    if (target !== state.current) {
      state.current = target;
      state.lastSwitchAt = now;
      state.lastProbeAt = now;
      return target;
    }
    return null;
  }

  // 2) Down-switch: the current rendition isn't getting through. Candidates
  //    are strictly below the current rendition — a lossy connection must
  //    never "down-switch" to a higher rung.
  if (lossy || starving) {
    let candidates: AbrRenditionInfo[];
    if (state.current === "source") {
      candidates = rungs;
    } else {
      const idx = rungs.findIndex((r) => r.name === state.current);
      candidates = idx > 0 ? rungs.slice(0, idx) : [];
    }
    let target: string | null = null;
    if (state.deliveredEma !== null) {
      target =
        fitRung(candidates, state.deliveredEma * FIT_HEADROOM)?.name ?? null;
    }
    // Unknown throughput: fall back to one rung down.
    if (!target && candidates.length > 0) {
      target = candidates[candidates.length - 1].name;
    }
    if (target && target !== state.current) {
      state.probeAt = null;
      state.probeFrom = null;
      state.current = target;
      state.lastSwitchAt = now;
      state.lastProbeAt = now;
      return target;
    }
    return null;
  }

  // 3) Up-probe: sustained clean delivery of the current rendition's full
  //    nominal rate means there might be headroom. One rung at a time.
  const wellFed =
    currentNominal !== null &&
    win.deliveredBps >= CLEAN_DELIVERY_RATIO * currentNominal &&
    clean;
  if (
    wellFed &&
    state.probeAt === null &&
    now >= state.upSuppressedUntil &&
    now - state.lastProbeAt >= MIN_PROBE_INTERVAL_MS
  ) {
    const next = rungAbove(rungs, state.current);
    if (next && next !== state.current) {
      state.probeFrom = state.current;
      state.probeAt = now;
      state.lastProbeAt = now;
      state.current = next;
      state.lastSwitchAt = now;
      return next;
    }
  }

  // A probe that survived past its eval window graduated: clear bookkeeping.
  if (state.probeAt !== null && now - state.probeAt > PROBE_EVAL_MS) {
    state.probeAt = null;
    state.probeFrom = null;
  }

  return null;
}
