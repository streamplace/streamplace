import { $Typed } from "@atproto/api";
import {
  Link as FacetLink,
  Mention,
} from "@atproto/api/dist/client/types/app/bsky/richtext/facet";
import type { LivestreamStore } from "@streamplace/core";
import { segmentize, type Facet, type FacetFeature } from "@streamplace/core";
import { ArrowDown, Reply } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import type {
  ChatMessageViewHydrated,
  PlaceStreamBadgeDefs,
} from "streamplace";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from "../ui/hover-card";

export function ChatPanel({ store }: { store: LivestreamStore }) {
  const { chat, authors, replyToMessage } = useStore(
    store,
    useShallow((s) => ({
      chat: s.chat,
      authors: s.authors,
      replyToMessage: s.replyToMessage,
    })),
  );

  const scrollRef = useRef<HTMLDivElement | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const [isAtBottom, setIsAtBottom] = useState(true);
  const [newMessageCount, setNewMessageCount] = useState(0);
  const prevChatLenRef = useRef(chat.length);

  // Track whether the user is scrolled to the bottom.
  const handleScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    const threshold = 40;
    setIsAtBottom(el.scrollHeight - el.scrollTop - el.clientHeight < threshold);
  }, []);

  // When new messages arrive, auto-scroll only if already at bottom.
  useEffect(() => {
    const delta = chat.length - prevChatLenRef.current;
    prevChatLenRef.current = chat.length;

    if (delta <= 0) return;

    if (isAtBottom) {
      messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
      setNewMessageCount(0);
    } else {
      setNewMessageCount((c) => c + delta);
    }
  }, [chat.length, isAtBottom]);

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
    setNewMessageCount(0);
  }, []);

  // Clear the badge when the user scrolls back to the bottom.
  useEffect(() => {
    if (isAtBottom) setNewMessageCount(0);
  }, [isAtBottom]);

  return (
    <div className="flex flex-col flex-1 min-h-0 max-w-full relative">
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto overflow-x-hidden p-3 space-y-0.5"
      >
        {chat.length === 0 ? (
          <div className="text-center text-[var(--color-fg-muted)] text-sm py-8">
            No messages yet
          </div>
        ) : (
          chat
            .slice(-100)
            .map((msg) => (
              <ChatMessage
                key={msg.uri}
                message={msg}
                profile={authors[msg.author.did]}
                authors={authors}
              />
            ))
        )}
        <div ref={messagesEndRef} />
      </div>

      {!isAtBottom && (
        <button
          onClick={scrollToBottom}
          className="absolute bottom-2 right-3 w-8 h-8 rounded-full bg-[var(--color-bg-elevated)] border border-[var(--color-border)] shadow-md flex items-center justify-center text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] hover:bg-[var(--color-bg-overlay)] transition-colors z-10"
          aria-label="Scroll to bottom"
        >
          <ArrowDown className="w-4 h-4" />
          {newMessageCount > 0 && (
            <span className="absolute -top-1 -right-1 min-w-[16px] h-4 px-1 rounded-full bg-[var(--color-accent)] text-white text-[10px] font-medium flex items-center justify-center">
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
}: {
  message: ChatMessageViewHydrated;
  profile: ChatMessageViewHydrated["chatProfile"];
  authors: { [key: string]: ChatMessageViewHydrated["chatProfile"] };
}) {
  const isSystem = message.author.did === "did:sys:system";

  if (isSystem) {
    return (
      <div className="py-1.5 px-2 rounded bg-[var(--color-bg-overlay)] border border-[var(--color-border)] my-1">
        <p className="text-sm text-center">{message.record.text}</p>
      </div>
    );
  }

  return (
    <div className="py-0.5 group hover:bg-[var(--color-bg-overlay)] rounded px-2 -mx-2 leading-snug">
      {message.replyTo && (
        <div className="pl-[60px]">
          <ReplyBanner parent={message.replyTo} authors={authors} />
        </div>
      )}

      <div className="flex items-start gap-2">
        <span className="text tabular-nums">
          {formatTime(message.record.createdAt)}
        </span>
        <div className="flex-1 min-w-0 flex-wrap items-center gap-1">
          {message.badges?.map((badge, i) => (
            <span className="items-end justify-end gap-0.5 inline-block">
              <BadgeIcon key={i} badge={badge} />
            </span>
          ))}

          <UserHandle
            author={message.author}
            color={profile?.color}
            badges={message.badges}
          />
          <span>{": "}</span>
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
              className="text-[var(--color-accent)] hover:underline break-all"
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
  const name = author.displayName || author.handle || author.did;
  const handle = author.handle;

  return (
    <HoverCard trigger="click">
      <HoverCardTrigger>
        <span
          className="font-medium cursor-pointer hover:underline"
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
        className="w-72 p-0 overflow-hidden"
      >
        {/* Banner — gradient tinted with the user's chat color */}
        <div
          className="h-20 relative"
          style={
            color
              ? {
                  background: `linear-gradient(135deg, rgb(${color.red}, ${color.green}, ${color.blue}) 0%, var(--color-muted) 100%)`,
                }
              : { background: "var(--color-muted)" }
          }
        />

        <div className="px-3 pt-1 relative">
          {/* Avatar overlapping the banner */}
          <img
            src={author.avatar ?? undefined}
            alt=""
            className="w-12 h-12 rounded-full border-2 border-[var(--color-bg-elevated)] bg-[var(--color-bg)] absolute -top-6 left-3"
            onError={(e) => {
              (e.currentTarget as HTMLImageElement).style.display = "none";
            }}
          />

          {/* Handle row */}
          <div className="pt-7 pb-2 flex items-center justify-between gap-2">
            <div className="min-w-0">
              {name !== handle && (
                <div
                  className="font-medium truncate"
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
                className="flex-shrink-0 text-[#0F73FF] hover:opacity-80"
                aria-label="View profile on Bluesky"
              >
                <BskyIcon size={20} />
              </a>
            )}
          </div>

          {/* Badges */}
          {badges && badges.length > 0 && (
            <div className="pb-2 space-y-1">
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
    <div className="flex items-center gap-1 text-[11px] text-[var(--color-fg-subtle)] mb-0.5 truncate">
      <Reply className="w-3 h-3 flex-shrink-0" />
      <span className="font-medium flex-shrink-0">@{handle}</span>
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

const BADGE_META: Record<string, { label: string; issuedBy: string }> = {
  "place.stream.badge.defs#mod": {
    label: "Moderator",
    issuedBy: "Issued by Streamplace",
  },
  "place.stream.badge.defs#streamer": {
    label: "Streamer",
    issuedBy: "Issued by Streamplace",
  },
  "place.stream.badge.defs#vip": {
    label: "VIP",
    issuedBy: "Issued by Streamplace",
  },
  "place.stream.badge.defs#bot": {
    label: "Bot",
    issuedBy: "Self-labeled",
  },
  "place.stream.badge.defs#event": {
    label: "Event",
    issuedBy: "Issued by Streamplace",
  },
};

function shortDid(did: string): string {
  // e.g. "did:plc:abc123def456" -> "did:plc:abc…"
  if (!did) return "";
  if (did.length <= 16) return did;
  return `${did.slice(0, 12)}…`;
}

function issuerLabel(badge: PlaceStreamBadgeDefs.BadgeView): string {
  // Service-issued badges come from the streamplace node itself.
  if (badge.issuer && badge.issuer.startsWith("did:web:")) {
    return "Issued by Streamplace";
  }
  if (badge.issuer) {
    return `Issued by ${shortDid(badge.issuer)}`;
  }
  return "Issued";
}

function BadgeIcon({ badge }: { badge: PlaceStreamBadgeDefs.BadgeView }) {
  const src = badge.imageUrl || BADGE_SRC[badge.badgeType];
  if (!src) return null;
  return (
    <img
      src={src}
      alt={badge.badgeType}
      className="inline-block w-4 h-4 rounded-xs align-middle relative top-0.5 mr-1"
    />
  );
}

function BadgeRow({ badge }: { badge: PlaceStreamBadgeDefs.BadgeView }) {
  const src = badge.imageUrl || BADGE_SRC[badge.badgeType];
  const meta = BADGE_META[badge.badgeType];
  // Prefer the badge def's own name (e.g. "Contest Winner"), then the
  // built-in label for known types, then a humanized fallback.
  const rawTag = badge.badgeType.split("#")[1] ?? "";
  const label =
    badge.name?.trim() ||
    meta?.label ||
    (rawTag ? rawTag[0].toUpperCase() + rawTag.slice(1) : "Badge");
  const issuedBy = meta?.issuedBy ?? issuerLabel(badge);

  return (
    <div className="flex items-center gap-2 p-2 rounded-md bg-[var(--color-bg-overlay)] border border-[var(--color-border)]">
      {src ? (
        <img
          src={src}
          alt={badge.badgeType}
          className="w-6 h-6 rounded-xs flex-shrink-0"
        />
      ) : (
        <div className="w-6 h-6 rounded-xs flex-shrink-0 bg-[var(--color-bg)]" />
      )}
      <div className="min-w-0 flex-1">
        <div className="font-medium leading-tight">{label}</div>
        <div className="text-sm text-[var(--color-fg-muted)] leading-tight">
          {issuedBy}
        </div>
      </div>
    </div>
  );
}
