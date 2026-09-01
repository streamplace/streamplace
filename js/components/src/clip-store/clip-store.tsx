import { SessionManager } from "@atproto/api/dist/session-manager";
import { place, StreamplaceAgent } from "streamplace";
import { create } from "zustand";
import { clampWindow, MIN_CLIP_MS } from "./trim-math";

export type ClipStatus =
  | "idle"
  | "requesting"
  | "editing"
  | "publishing"
  | "published"
  | "expired"
  | "error";

export interface ClipState {
  status: ClipStatus;
  clipId: string | null;
  previewUrl: string | null;
  expiresAt: number | null;
  durationMs: number;
  trimStart: number;
  trimEnd: number;
  title: string;
  videoUri: string | null;
  clipUri: string | null;
  error: string | null;
  /** Milliseconds until the draft expires; ticked every second while editing. */
  timeRemaining: number;
  /**
   * Internal: snapshotted at draft creation. The oauth session and livestream
   * URI live in per-provider context stores (not module singletons), so the
   * draft captures its agent + livestream here — publish() stays argument-free
   * and the draft survives navigation away from the player.
   */
  agent: StreamplaceAgent | null;
  livestreamUri: string | null;
}

export interface ClipStore extends ClipState {
  requestClip: (input: {
    streamerDID: string;
    oauthSession: SessionManager | null | undefined;
    livestreamUri: string | null;
    durationMs?: number;
  }) => Promise<void>;
  setTrim: (start: number, end: number) => void;
  setTitle: (title: string) => void;
  publish: () => Promise<void>;
  /** Best-effort server cancel: deletes the ephemeral draft file, then resets. */
  cancel: () => Promise<void>;
  discard: () => void;
  /** Called every second while editing; flips editing → expired at TTL. */
  tick: (now?: number) => void;
}

const initialState: ClipState = {
  status: "idle",
  clipId: null,
  previewUrl: null,
  expiresAt: null,
  durationMs: 0,
  trimStart: 0,
  trimEnd: 0,
  title: "",
  videoUri: null,
  clipUri: null,
  error: null,
  timeRemaining: 0,
  agent: null,
  livestreamUri: null,
};

export type ClipErrorKind = "create" | "publish";

const CREATE_ERROR_COPY: Record<string, string> = {
  ClippingDisabled: "The streamer has disabled clipping",
  RateLimited: "You're clipping too fast, try again in a moment",
  NotLive: "The stream isn't live right now",
  NoContent: "Nothing to clip yet, try again in a moment",
  Unauthorized: "Please sign in to clip",
};

const PUBLISH_ERROR_COPY: Record<string, string> = {
  DraftNotFound: "This clip draft no longer exists, start over",
  DraftExpired: "This clip draft expired, start over",
  Unauthorized: "Please sign in to publish",
};

// Map a lexicon error name (the @atproto/lex client throws XrpcResponseError
// with `.error` = error name and `.message` = server description) to
// user-facing copy, falling back to the server message for unknown names.
export function mapClipError(kind: ClipErrorKind, error: unknown): string {
  const name = (error as any)?.error;
  if (typeof name === "string") {
    const copy = (kind === "create" ? CREATE_ERROR_COPY : PUBLISH_ERROR_COPY)[
      name
    ];
    if (copy) return copy;
  }
  const message = (error as any)?.message;
  if (typeof message === "string" && message.trim()) return message;
  return kind === "create" ? "Failed to create clip" : "Failed to publish clip";
}

let expiryTimer: ReturnType<typeof setInterval> | null = null;

function startExpiryTimer() {
  if (expiryTimer !== null) return;
  expiryTimer = setInterval(() => {
    useClipStore.getState().tick();
  }, 1000);
}

function clearExpiryTimer() {
  if (expiryTimer !== null) {
    clearInterval(expiryTimer);
    expiryTimer = null;
  }
}

export const useClipStore = create<ClipStore>()((set, get) => ({
  ...initialState,

  requestClip: async ({
    streamerDID,
    oauthSession,
    livestreamUri,
    durationMs = 120000,
  }) => {
    // Branch on the three oauthSession states the same way usePDSAgent does:
    // undefined = session still loading, null = logged out, SessionManager =
    // logged in. The trigger only renders the login CTA when logged out, so
    // these guards are defensive against boot-time races.
    if (oauthSession === undefined) {
      set({ status: "error", error: "Still loading your session" });
      return;
    }
    if (oauthSession === null) {
      set({ status: "error", error: "Please sign in to clip" });
      return;
    }
    const agent = new StreamplaceAgent(oauthSession);
    set({ status: "requesting", error: null });
    try {
      const result = await agent.client.call(place.stream.clip.create, {
        streamer: streamerDID as any,
        durationMs,
      });
      const expiresAt = new Date(result.expiresAt).getTime();
      set({
        status: "editing",
        clipId: result.clipId,
        previewUrl: result.previewUrl,
        expiresAt,
        durationMs: result.durationMs,
        // Default trim: the full grabbed duration. result.durationMs is
        // authoritative (may be less than requested if the buffer was short).
        trimStart: 0,
        trimEnd: result.durationMs,
        title: "",
        videoUri: null,
        clipUri: null,
        error: null,
        timeRemaining: Math.max(0, expiresAt - Date.now()),
        agent,
        livestreamUri,
      });
      startExpiryTimer();
    } catch (e) {
      set({ status: "error", error: mapClipError("create", e) });
    }
  },

  setTrim: (start, end) => {
    const s = get();
    if (s.status !== "editing") return;
    const clamped = clampWindow(start, end, s.durationMs, MIN_CLIP_MS);
    // The XRPC lexicon types start/end as int64 milliseconds. Timeline drags
    // produce float ms via px→ms conversion, so round here — the store is the
    // only writer of trim state and publish() sends these values verbatim.
    set({
      trimStart: Math.round(clamped.start),
      trimEnd: Math.round(clamped.end),
    });
  },

  setTitle: (title) => set({ title }),

  publish: async () => {
    const s = get();
    // Guards run before any request is sent. An expired draft must never
    // publish — the server's DraftExpired error is a backstop, not the primary
    // guard. A publish failure leaves status "error" with the clipId intact so
    // the editor can retry.
    if (s.status === "publishing") return;
    if (s.status === "expired") return;
    if (!s.clipId || !s.agent || !s.livestreamUri) {
      set({
        status: "error",
        error: "This clip draft is no longer available, start over",
      });
      return;
    }
    if (!s.title.trim()) {
      set({ error: "Title is required" });
      return;
    }
    set({ status: "publishing", error: null });
    const clipId = s.clipId;
    try {
      const result = await s.agent.client.call(place.stream.clip.publish, {
        clipId: s.clipId,
        livestream: s.livestreamUri as any,
        start: s.trimStart,
        end: s.trimEnd,
        title: s.title,
      });
      // The draft may have been discarded (dialog X/backdrop) while the
      // request was in flight — don't resurrect it.
      if (get().clipId !== clipId) return;
      clearExpiryTimer();
      set({
        status: "published",
        videoUri: result.videoUri,
        clipUri: result.clipUri ?? null,
      });
    } catch (e) {
      if (get().clipId !== clipId) return;
      set({ status: "error", error: mapClipError("publish", e) });
    }
  },

  cancel: async () => {
    const s = get();
    // Best-effort server cleanup: delete the ephemeral draft file now instead
    // of waiting for the 10-minute TTL sweep. Failures are ignored — the
    // sweep is the backstop. Published drafts have no ephemeral file left.
    if (s.clipId && s.agent && s.status !== "published") {
      try {
        await s.agent.client.call(place.stream.clip.cancel, {
          clipId: s.clipId,
        });
      } catch {
        // ignore — the TTL sweep cleans up eventually
      }
    }
    clearExpiryTimer();
    set(initialState);
  },

  discard: () => {
    clearExpiryTimer();
    set(initialState);
  },

  tick: (now = Date.now()) => {
    const s = get();
    if (s.status !== "editing" || s.expiresAt === null) return;
    const remaining = Math.max(0, s.expiresAt - now);
    if (remaining <= 0) {
      clearExpiryTimer();
      set({ status: "expired", timeRemaining: 0 });
    } else {
      set({ timeRemaining: remaining });
    }
  },
}));
