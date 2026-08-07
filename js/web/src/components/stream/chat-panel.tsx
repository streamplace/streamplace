import { $Typed } from "@atproto/api";
import {
  Link as FacetLink,
  Mention,
} from "@atproto/api/dist/client/types/app/bsky/richtext/facet";
import type { LivestreamStore } from "@streamplace/core";
import { segmentize, type Facet, type FacetFeature } from "@streamplace/core";
import { ArrowDown, ArrowUp, Pin, Reply } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type {
  ChatMessageViewHydrated,
  PlaceStreamBadgeDefs,
} from "streamplace";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import { useSession } from "../../lib/session";
import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from "../ui/hover-card";

export function ChatPanel({
  store,
  reversed = false,
}: {
  store: LivestreamStore;
  reversed?: boolean;
}) {
  const { t } = useTranslation("common");
  const { chat, authors, replyToMessage, websocketConnected } = useStore(
    store,
    useShallow((s) => ({
      chat: s.chat,
      authors: s.authors,
      replyToMessage: s.replyToMessage,
      websocketConnected: s.websocketConnected,
    })),
  );

  const scrollRef = useRef<HTMLDivElement | null>(null);
  const anchorRef = useRef<HTMLDivElement | null>(null);
  const [isAtAnchor, setIsAtAnchor] = useState(true);
  const [newMessageCount, setNewMessageCount] = useState(0);
  const prevChatLenRef = useRef(chat.length);

  // Track whether the user is scrolled to the anchor end.
  // Normal: anchor is bottom. Reversed: anchor is top.
  const handleScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    const threshold = 40;
    if (reversed) {
      setIsAtAnchor(el.scrollTop < threshold);
    } else {
      setIsAtAnchor(
        el.scrollHeight - el.scrollTop - el.clientHeight < threshold,
      );
    }
  }, [reversed]);

  // When new messages arrive, auto-scroll only if already at anchor.
  useEffect(() => {
    const delta = chat.length - prevChatLenRef.current;
    prevChatLenRef.current = chat.length;

    if (delta <= 0) return;

    if (isAtAnchor) {
      anchorRef.current?.scrollIntoView({ behavior: "smooth" });
      setNewMessageCount(0);
    } else {
      setNewMessageCount((c) => c + delta);
    }
  }, [chat.length, isAtAnchor]);

  const scrollToAnchor = useCallback(() => {
    anchorRef.current?.scrollIntoView({ behavior: "smooth" });
    setNewMessageCount(0);
  }, []);

  // Clear the badge when the user scrolls back to the anchor.
  useEffect(() => {
    if (isAtAnchor) setNewMessageCount(0);
  }, [isAtAnchor]);

  const displayMessages = useMemo(() => {
    const sliced = chat.slice(-1500);
    return reversed ? [...sliced].reverse() : sliced;
  }, [chat, reversed]);

  return (
    <div className="relative flex min-h-0 max-w-full flex-1 flex-col">
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="flex-1 space-y-0.5 overflow-x-hidden overflow-y-auto p-3"
      >
        {reversed && <div ref={anchorRef} />}
        {displayMessages.length === 0 ? (
          !websocketConnected ? (
            <ChatSkeleton />
          ) : (
            <div className="py-8 text-center text-sm text-(--color-fg-muted)">
              {t("chat-no-messages")}
            </div>
          )
        ) : (
          displayMessages.map((msg, i) => {
            const prev = i > 0 ? displayMessages[i - 1] : null;
            const isGrouped =
              !!prev &&
              prev.author.did === msg.author.did &&
              !prev.author.did.startsWith("did:sys:") &&
              !msg.author.did.startsWith("did:sys:") &&
              !msg.replyTo &&
              // Only group within a reasonable time window (5 minutes)
              Math.abs(
                new Date(msg.record.createdAt).getTime() -
                  new Date(prev.record.createdAt).getTime(),
              ) <
                5 * 60 * 1000;

            return (
              <ChatMessage
                key={msg.uri}
                message={msg}
                profile={authors[msg.author.did]}
                authors={authors}
                store={store}
                isGrouped={isGrouped}
              />
            );
          })
        )}
        {!reversed && <div ref={anchorRef} />}
      </div>

      {!isAtAnchor && (
        <button
          onClick={scrollToAnchor}
          className="absolute right-3 bottom-2 z-10 flex h-8 w-8 items-center justify-center rounded-full border border-(--color-border) bg-(--color-bg-elevated) text-(--color-fg-muted) shadow-md transition-colors hover:bg-(--color-bg-overlay) hover:text-(--color-fg)"
          aria-label={
            reversed ? t("chat-scroll-to-top") : t("chat-scroll-to-bottom")
          }
        >
          {reversed ? (
            <ArrowUp className="h-4 w-4" />
          ) : (
            <ArrowDown className="h-4 w-4" />
          )}
          {newMessageCount > 0 && (
            <span className="absolute -top-1 -right-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-(--color-accent) px-1 text-[10px] font-medium text-white">
              {newMessageCount}
            </span>
          )}
        </button>
      )}
    </div>
  );
}

function ChatMessage({
  message,
  profile,
  authors,
  store,
  isGrouped = false,
}: {
  message: ChatMessageViewHydrated;
  profile: ChatMessageViewHydrated["chatProfile"];
  authors: { [key: string]: ChatMessageViewHydrated["chatProfile"] };
  store: LivestreamStore;
  isGrouped?: boolean;
}) {
  const { t } = useTranslation("common");
  const { state, pdsAgent, did } = useSession();
  const isSystem = message.author.did === "did:sys:system";
  const isOwn = did === message.author.did;

  // Check if current user is the streamer (can pin messages).
  const streamerDid = useStore(store, (s) => s.livestream?.author.did);
  const canPin = did && streamerDid && did === streamerDid;

  const handleReply = useCallback(() => {
    store.setState((s) => ({ ...s, replyToMessage: message }));
  }, [store, message]);

  const handlePin = useCallback(async () => {
    if (!pdsAgent || !streamerDid) return;
    try {
      await pdsAgent.com.atproto.repo.createRecord({
        repo: streamerDid,
        collection: "place.stream.chat.pinnedRecord",
        record: {
          $type: "place.stream.chat.pinnedRecord",
          pinnedMessage: message.uri,
          createdAt: new Date().toISOString(),
        },
      });
    } catch (e) {
      console.error("Failed to pin message:", e);
    }
  }, [pdsAgent, streamerDid, message.uri]);

  if (isSystem) {
    return (
      <div className="my-1 rounded border border-(--color-border) bg-(--color-bg-overlay) px-2 py-1.5">
        <p className="text-center text-sm">{message.record.text}</p>
      </div>
    );
  }

  return (
    <div
      className={`group relative -mx-2 rounded px-2 leading-snug hover:bg-(--color-bg-overlay) ${isGrouped ? "py-px" : "py-0.5"}`}
    >
      {/* Hover actions — visible on group hover */}
      <div className="absolute top-0 right-0 z-10 flex -translate-y-1/2 items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
        {state.status === "authenticated" && !isOwn && (
          <button
            type="button"
            onClick={handleReply}
            className="rounded border border-(--color-border) bg-(--color-bg-elevated) p-1 text-(--color-fg-muted) shadow-sm transition-colors hover:bg-(--color-bg-overlay) hover:text-(--color-fg)"
            aria-label={t("chat-reply-to-message")}
          >
            <Reply className="h-3.5 w-3.5" />
          </button>
        )}
        {canPin && (
          <button
            type="button"
            onClick={handlePin}
            className="rounded border border-(--color-border) bg-(--color-bg-elevated) p-1 text-(--color-fg-muted) shadow-sm transition-colors hover:bg-(--color-bg-overlay) hover:text-(--color-accent)"
            aria-label={t("chat-pin-message")}
          >
            <Pin className="h-3.5 w-3.5" />
          </button>
        )}
      </div>

      {message.replyTo && (
        <div className="flex gap-2 pl-2">
          <div className="w-0.5 shrink-0 rounded-full bg-(--color-accent)/30" />
          <div className="min-w-0 flex-1">
            <ReplyBanner parent={message.replyTo} authors={authors} />
          </div>
        </div>
      )}

      <div
        className={`flex items-start gap-2 ${isGrouped ? "pl-17" : ""} ${message.replyTo ? "pl-5" : ""}`}
      >
        {!isGrouped && (
          <span className="text tabular-nums">
            {formatTime(message.record.createdAt)}
          </span>
        )}
        <div className="min-w-0 flex-1 flex-wrap items-center gap-1">
          {!isGrouped &&
            message.badges?.map((badge, i) => (
              <span
                key={i}
                className="inline-block items-end justify-end gap-0.5"
              >
                <BadgeIcon key={i} badge={badge} />
              </span>
            ))}

          {!isGrouped && (
            <UserHandle
              author={message.author}
              color={profile?.color}
              badges={message.badges}
            />
          )}
          {!isGrouped && <span>{": "}</span>}
          <RichTextMessage
            text={message.record.text}
            facets={message.record.facets}
            authors={authors}
          />
        </div>
      </div>
    </div>
  );
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  return d.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

function RichTextMessage({
  text,
  facets,
  authors,
}: {
  text: string;
  facets: ChatMessageViewHydrated["record"]["facets"];
  authors: { [key: string]: ChatMessageViewHydrated["chatProfile"] };
}) {
  if (!facets || facets.length === 0) {
    return <span className="">{text}</span>;
  }
  const segs = segmentize(text, facets as Facet[]);
  return (
    <>
      {segs.map((seg, i) => {
        const ftr = seg.features?.[0] as FacetFeature | undefined;
        if (!ftr) {
          return (
            <span key={i} className="">
              {seg.text}
            </span>
          );
        }
        if (ftr.$type === "app.bsky.richtext.facet#link") {
          const linkFtr = ftr as $Typed<FacetLink>;
          return (
            <a
              key={i}
              href={linkFtr.uri}
              target="_blank"
              rel="noopener noreferrer"
              className="break-all text-(--color-accent) hover:underline"
            >
              {seg.text}
            </a>
          );
        }
        if (ftr.$type === "app.bsky.richtext.facet#mention") {
          const mtnFtr = ftr as $Typed<Mention>;
          const profile = authors?.[mtnFtr.did];
          const color = profile?.color;
          return (
            <a
              key={i}
              href={
                mtnFtr.did
                  ? `https://bsky.app/profile/${mtnFtr.did}`
                  : undefined
              }
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium hover:underline"
              style={
                color
                  ? {
                      color: `rgb(${color.red}, ${color.green}, ${color.blue})`,
                    }
                  : undefined
              }
            >
              {seg.text}
            </a>
          );
        }
        return (
          <span key={i} className="">
            {seg.text}
          </span>
        );
      })}
    </>
  );
}

function UserHandle({
  author,
  color,
  badges,
}: {
  author: ChatMessageViewHydrated["author"];
  color: { red: number; green: number; blue: number } | undefined;
  badges?: PlaceStreamBadgeDefs.BadgeView[];
}) {
  const { t } = useTranslation("common");
  const name = author.displayName || author.handle || author.did;
  const handle = author.handle;

  return (
    <HoverCard trigger="click">
      <HoverCardTrigger>
        <span
          className="cursor-pointer font-medium hover:underline"
          style={
            color
              ? { color: `rgb(${color.red}, ${color.green}, ${color.blue})` }
              : undefined
          }
        >
          {name}
        </span>
      </HoverCardTrigger>
      <HoverCardContent
        side="top"
        align="start"
        className="w-72 overflow-hidden p-0"
      >
        {/* Banner — gradient tinted with the user's chat color */}
        <div
          className="relative h-20"
          style={
            color
              ? {
                  background: `linear-gradient(135deg, rgb(${color.red}, ${color.green}, ${color.blue}) 0%, var(--color-muted) 100%)`,
                }
              : { background: "var(--color-muted)" }
          }
        />

        <div className="relative px-3 pt-1">
          {/* Avatar overlapping the banner */}
          <img
            src={author.avatar ?? undefined}
            alt=""
            className="absolute -top-6 left-3 h-12 w-12 rounded-full border-2 border-(--color-bg-elevated) bg-(--color-bg)"
            onError={(e) => {
              (e.currentTarget as HTMLImageElement).style.display = "none";
            }}
          />

          {/* Handle row */}
          <div className="flex items-center justify-between gap-2 pt-7 pb-2">
            <div className="min-w-0">
              {name !== handle && (
                <div
                  className="truncate font-medium"
                  style={
                    color
                      ? {
                          color: `rgb(${color.red}, ${color.green}, ${color.blue})`,
                        }
                      : undefined
                  }
                >
                  {name}
                </div>
              )}
              {handle && <div className="truncate">@{handle}</div>}
            </div>
            {handle && (
              <a
                href={`https://bsky.app/profile/${handle}`}
                target="_blank"
                rel="noopener noreferrer"
                className="shrink-0 text-[#0F73FF] hover:opacity-80"
                aria-label={t("chat-view-profile-bluesky")}
              >
                <BskyIcon size={20} />
              </a>
            )}
          </div>

          {/* Badges */}
          {badges && badges.length > 0 && (
            <div className="space-y-1 pb-2">
              {badges.map((badge, i) => (
                <BadgeRow key={i} badge={badge} />
              ))}
            </div>
          )}
        </div>
      </HoverCardContent>
    </HoverCard>
  );
}

function BskyIcon({ size = 24 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 64 57"
      fill="#0F73FF"
      style={{ aspectRatio: "64 / 57", display: "inline-block" }}
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M13.873 3.805C21.21 9.332 29.103 20.537 32 26.55v15.882c0-.338-.13.044-.41.867-1.512 4.456-7.418 21.847-20.923 7.944-7.111-7.32-3.819-14.64 9.125-16.85-7.405 1.264-15.73-.825-18.014-9.015C1.12 23.022 0 8.51 0 6.55 0-3.268 8.579-.182 13.873 3.805ZM50.127 3.805C42.79 9.332 34.897 20.537 32 26.55v15.882c0-.338.13.044.41.867 1.512 4.456 7.418 21.847 20.923 7.944 7.111-7.32 3.819-14.64-9.125-16.85 7.405 1.264 15.73-.825 18.014-9.015C62.88 23.022 64 8.51 64 6.55c0-9.818-8.578-6.732-13.873-2.745Z"
      />
    </svg>
  );
}

function ReplyBanner({
  parent,
  authors,
}: {
  parent: ChatMessageViewHydrated["replyTo"];
  authors: { [key: string]: ChatMessageViewHydrated["chatProfile"] };
}) {
  if (!parent) return null;
  if (parent.$type !== "place.stream.chat.message") return null;
  const msg = parent as ChatMessageViewHydrated;
  const text = msg.record?.text || "";
  const handle = msg.author?.handle || "...";
  const facets = msg.record?.facets as
    | ChatMessageViewHydrated["record"]["facets"]
    | undefined;

  return (
    <div className="mb-0.5 flex items-center gap-1 truncate text-[11px] text-(--color-fg-subtle)">
      <Reply className="h-3 w-3 shrink-0" />
      <span className="shrink-0 font-medium">@{handle}</span>
      <span className="truncate">
        <RichTextMessage text={text} facets={facets} authors={authors} />
      </span>
    </div>
  );
}

const BADGE_SRC: Record<string, string> = {
  "place.stream.badge.defs#mod": "/mod_2x.png",
  "place.stream.badge.defs#streamer": "/live_2x.png",
  "place.stream.badge.defs#vip": "/vip_2x.png",
  "place.stream.badge.defs#bot": "/robot_2x.png",
};

const BADGE_I18N_KEYS: Record<string, string> = {
  "place.stream.badge.defs#mod": "badge-moderator",
  "place.stream.badge.defs#streamer": "badge-streamer",
  "place.stream.badge.defs#vip": "badge-vip",
  "place.stream.badge.defs#bot": "badge-bot",
  "place.stream.badge.defs#event": "badge-event",
};

function shortDid(did: string): string {
  // e.g. "did:plc:abc123def456" -> "did:plc:abc…"
  if (!did) return "";
  if (did.length <= 16) return did;
  return `${did.slice(0, 12)}…`;
}

function issuerLabel(
  badge: PlaceStreamBadgeDefs.BadgeView,
  t: (key: string, opts?: Record<string, unknown>) => string,
): string {
  if (badge.issuer && badge.issuer.startsWith("did:web:")) {
    return t("badge-issued-by-streamplace");
  }
  if (badge.issuer) {
    return t("badge-issued-by", { issuer: shortDid(badge.issuer) });
  }
  return t("badge-issued");
}

function BadgeIcon({ badge }: { badge: PlaceStreamBadgeDefs.BadgeView }) {
  const src = badge.imageUrl || BADGE_SRC[badge.badgeType];
  if (!src) return null;
  return (
    <img
      src={src}
      alt={badge.badgeType}
      className="relative top-0.5 mr-1 inline-block h-4 w-4 rounded-xs align-middle"
    />
  );
}

function BadgeRow({ badge }: { badge: PlaceStreamBadgeDefs.BadgeView }) {
  const { t } = useTranslation("common");
  const src = badge.imageUrl || BADGE_SRC[badge.badgeType];
  const i18nKey = BADGE_I18N_KEYS[badge.badgeType];
  const rawTag = badge.badgeType.split("#")[1] ?? "";
  const label =
    badge.name?.trim() ||
    (i18nKey ? t(i18nKey) : null) ||
    (rawTag ? rawTag[0].toUpperCase() + rawTag.slice(1) : t("badge-fallback"));
  const isSelfLabeled = badge.badgeType === "place.stream.badge.defs#bot";
  const issuedBy = isSelfLabeled
    ? t("badge-self-labeled")
    : issuerLabel(badge, t);

  return (
    <div className="flex items-center gap-2 rounded-md border border-(--color-border) bg-(--color-bg-overlay) p-2">
      {src ? (
        <img
          src={src}
          alt={badge.badgeType}
          className="h-6 w-6 shrink-0 rounded-xs"
        />
      ) : (
        <div className="h-6 w-6 shrink-0 rounded-xs bg-(--color-bg)" />
      )}
      <div className="min-w-0 flex-1">
        <div className="leading-tight font-medium">{label}</div>
        <div className="text-sm leading-tight text-(--color-fg-muted)">
          {issuedBy}
        </div>
      </div>
    </div>
  );
}

function ChatSkeleton() {
  const lines = [
    { handle: 16, msg: 60 },
    { handle: 20, msg: 80 },
    { handle: 14, msg: 45 },
    { handle: 18, msg: 70 },
    { handle: 12, msg: 55 },
  ];

  return (
    <div className="space-y-2 p-3">
      {lines.map((line, i) => (
        <div key={i} className="flex items-start gap-2">
          <span className="h-3 w-10 shrink-0 animate-pulse rounded bg-(--color-bg-elevated)" />
          <div className="flex flex-1 flex-wrap items-center gap-1">
            <span
              className="h-3 animate-pulse rounded bg-(--color-bg-elevated)"
              style={{ width: `${line.handle * 0.5}rem` }}
            />
            <span
              className="h-3 animate-pulse rounded bg-(--color-bg-elevated)"
              style={{ width: `${line.msg * 0.45}rem` }}
            />
          </div>
        </div>
      ))}
    </div>
  );
}
