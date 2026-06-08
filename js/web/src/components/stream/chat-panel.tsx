import { $Typed } from "@atproto/api";
import {
  Link as FacetLink,
  Mention,
} from "@atproto/api/dist/client/types/app/bsky/richtext/facet";
import type { LivestreamStore } from "@streamplace/core";
import { segmentize, type Facet, type FacetFeature } from "@streamplace/core";
import { Link } from "@tanstack/react-router";
import { Reply, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type {
  ChatMessageViewHydrated,
  PlaceStreamBadgeDefs,
} from "streamplace";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import { useChatSend } from "../../hooks/use-chat-send";
import { EMPTY_LOGIN_SEARCH } from "../../lib/login-search";
import { useSession } from "../../lib/session";
import { Button } from "../ui/button";
import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from "../ui/hover-card";
import { Popover, PopoverContent } from "../ui/popover";

export function ChatPanel({ store }: { store: LivestreamStore }) {
  const { chat, authors, replyToMessage } = useStore(
    store,
    useShallow((s) => ({
      chat: s.chat,
      authors: s.authors,
      replyToMessage: s.replyToMessage,
    })),
  );

  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [chat.length]);

  return (
    <div className="flex flex-col flex-1 min-h-0 max-w-full">
      <div className="flex-1 overflow-y-auto p-3 space-y-0.5">
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
        <div className="flex-1 min-w-0">
          {message.badges?.map((badge, i) => (
            <BadgeIcon key={i} badge={badge} />
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
              className="text-[var(--color-accent)] hover:underline"
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
      className="inline-block w-4 h-4 rounded-xs align-middle relative -top-px mr-1"
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

function useMentionState(value: string, chat: ChatMessageViewHydrated[]) {
  const cursorPos = useRef(0);
  const [selectedIndex, setSelectedIndex] = useState(0);

  const onInput = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    cursorPos.current = e.target.selectionStart || 0;
    setSelectedIndex(0);
  }, []);

  const mentionActive = useMemo(() => {
    const pos = cursorPos.current;
    if (pos === 0) return null;
    const before = value.slice(0, pos);
    const atIdx = before.lastIndexOf("@");
    if (atIdx === -1) return null;
    const after = before.slice(atIdx + 1);
    if (after.includes(" ")) return null;
    return { query: after.toLowerCase(), start: atIdx };
  }, [value]);

  const suggestions = useMemo(() => {
    if (!mentionActive) return [];
    const seen = new Set<string>();
    const matches: ChatMessageViewHydrated["author"][] = [];
    for (const msg of chat) {
      const did = msg.author.did;
      if (seen.has(did)) continue;
      seen.add(did);
      const handle = (msg.author.handle || "").toLowerCase();
      const display = (msg.author.displayName || "").toLowerCase();
      if (
        handle.includes(mentionActive.query) ||
        display.includes(mentionActive.query)
      ) {
        matches.push(msg.author);
      }
    }
    return matches.slice(0, 8);
  }, [chat, mentionActive]);

  const insertMention = useCallback(
    (author: ChatMessageViewHydrated["author"]) => {
      if (!mentionActive) return value;
      const before = value.slice(0, mentionActive.start);
      const after = value.slice(cursorPos.current);
      return `${before}@${author.handle} ${after}`;
    },
    [value, mentionActive],
  );

  return {
    mentionActive,
    suggestions,
    selectedIndex,
    setSelectedIndex,
    onInput,
    insertMention,
  };
}

export function ChatInput({ store }: { store: LivestreamStore }) {
  const { state } = useSession();
  const isAuthed = state.status === "authenticated";
  const send = useChatSend(store);
  const replyToMessage = useStore(store, (s) => s.replyToMessage);
  const chat = useStore(store, (s) => s.chat);
  const [value, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const {
    mentionActive,
    suggestions,
    selectedIndex,
    setSelectedIndex,
    onInput,
    insertMention,
  } = useMentionState(value, chat);

  const trimmed = value.trim();

  const onSubmit = useCallback(async () => {
    if (!trimmed || sending) return;
    setError(null);
    setSending(true);
    try {
      await send(trimmed);
      setValue("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to send message");
    } finally {
      setSending(false);
    }
  }, [trimmed, sending, send]);

  const onKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (!mentionActive || suggestions.length === 0) return;
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setSelectedIndex((i) => (i + 1) % suggestions.length);
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setSelectedIndex(
          (i) => (i - 1 + suggestions.length) % suggestions.length,
        );
      } else if (e.key === "Enter" || e.key === "Tab") {
        e.preventDefault();
        const author = suggestions[selectedIndex];
        if (author) {
          setValue(insertMention(author));
        }
      } else if (e.key === "Escape") {
        e.preventDefault();
        setSelectedIndex(0);
      }
    },
    [
      mentionActive,
      suggestions,
      selectedIndex,
      setSelectedIndex,
      insertMention,
    ],
  );

  if (!isAuthed) {
    return (
      <div className="text-sm text-[var(--color-fg-muted)] text-center py-1">
        <Link
          to="/login"
          search={EMPTY_LOGIN_SEARCH}
          className="text-[var(--color-accent)] hover:underline font-medium"
        >
          Log in
        </Link>{" "}
        to chat
      </div>
    );
  }

  return (
    <div>
      {replyToMessage && (
        <div className="flex items-center gap-2 px-2 py-1 mb-1 rounded bg-[var(--color-bg-overlay)] border border-[var(--color-border)] text-xs">
          <Reply className="w-3 h-3 text-[var(--color-fg-muted)] flex-shrink-0" />
          <span className="text-[var(--color-fg-muted)] flex-1 truncate">
            Replying to{" "}
            <span className="font-medium">
              {replyToMessage.author.handle || replyToMessage.author.did}
            </span>
          </span>
          <button
            type="button"
            onClick={() =>
              store.setState((s) => ({ ...s, replyToMessage: null }))
            }
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          >
            <X className="w-3 h-3" />
          </button>
        </div>
      )}

      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (mentionActive && suggestions.length > 0) return;
          onSubmit();
        }}
        className="flex gap-2"
      >
        <Popover open={!!mentionActive && suggestions.length > 0}>
          <div className="flex-1 relative">
            <input
              ref={inputRef}
              type="text"
              value={value}
              onChange={(e) => {
                setValue(e.target.value);
                onInput(e);
              }}
              onKeyDown={onKeyDown}
              placeholder="Send a message"
              maxLength={300}
              className="w-full h-9 px-3 pr-10 rounded-md bg-[var(--color-bg)] border border-[var(--color-border)] focus:border-[var(--color-accent)] focus:outline-none placeholder:text-[var(--color-fg-subtle)]"
              disabled={sending}
              aria-label="Chat message"
            />
            {value.length > 200 && (
              <span
                className={`absolute right-2 top-1/2 -translate-y-1/2 text-[10px] tabular-nums ${value.length >= 300 ? "text-[var(--color-danger)]" : "text-[var(--color-fg-subtle)]"}`}
              >
                {300 - value.length}
              </span>
            )}

            <PopoverContent
              align="start"
              className="w-full max-h-48 overflow-y-auto p-1"
            >
              {suggestions.map((author, i) => (
                <button
                  key={author.did}
                  type="button"
                  className={`flex items-center gap-2 w-full px-2 py-1.5 rounded text-left ${i === selectedIndex ? "bg-[var(--color-bg-overlay)]" : ""}`}
                  onClick={() => setValue(insertMention(author))}
                  onMouseEnter={() => setSelectedIndex(i)}
                >
                  <img
                    src={author.avatar ?? undefined}
                    alt=""
                    className="w-5 h-5 rounded-full bg-[var(--color-bg)] flex-shrink-0"
                    onError={(e) => {
                      (e.currentTarget as HTMLImageElement).style.display =
                        "none";
                    }}
                  />
                  <span className="font-medium truncate">
                    {author.displayName || author.handle}
                  </span>
                  <span className="text-[var(--color-fg-muted)] text-xs flex-shrink-0">
                    @{author.handle}
                  </span>
                </button>
              ))}
            </PopoverContent>
          </div>
        </Popover>

        <Button type="submit" size="sm" disabled={sending || trimmed === ""}>
          {sending ? "…" : "Chat"}
        </Button>
      </form>

      {error && (
        <div className="text-xs text-[var(--color-danger)] mt-1">{error}</div>
      )}
    </div>
  );
}
