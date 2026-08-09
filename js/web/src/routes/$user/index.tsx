import { ChatSidebar } from "@/components/stream/chat-sidebar";
import {
  activityLabel,
  CopyButton,
  StreamInfo,
} from "@/components/stream/stream-info";
import { VideoSection } from "@/components/stream/video-section";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { useFullscreen } from "@/contexts/fullscreen-context";
import { useLivenessState } from "@/hooks/use-liveness-state";
import { useLivestreamStore } from "@/hooks/use-livestream-store";
import { useStreamAvatar } from "@/hooks/use-stream-avatar";
import { useStreamplaceUrl } from "@/lib/store/hooks";
import type { LivestreamStore } from "@streamplace/core";
import { createFileRoute } from "@tanstack/react-router";
import { ExternalLink, Info, MessageCircle, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import {
  chatOpenAfterLivenessChange,
  chatPreferenceKey,
  shouldOpenChat,
} from "../../components/stream/chat-preference";
import { StreamAvatar } from "../../components/stream/stream-avatar";

export const Route = createFileRoute("/$user/")({
  component: StreamPage,
});

function StreamPage() {
  const { user } = Route.useParams();
  const { store, ready } = useLivestreamStore(user);

  if (!ready || !store) {
    return (
      <div className="mx-auto max-w-6xl px-6 py-12">
        <div className="animate-pulse">
          <div className="mb-4 h-8 w-48 rounded bg-(--color-bg-elevated)" />
          <div className="aspect-video rounded bg-(--color-bg-elevated)" />
        </div>
      </div>
    );
  }

  return <StreamBody store={store} user={user} />;
}

function StreamBody({ store, user }: { store: LivestreamStore; user: string }) {
  const liveness = useLivenessState(store);
  const avatar = useStreamAvatar(store);
  const { theatre } = useFullscreen();
  // Offline streams default to a closed chat; the chat has nothing
  // to show, and the shorter offline player should take the page
  // width. Live streams honor the user's saved preference.
  const isOffline = liveness === "offline" || liveness === "never-live";
  const preferredChatOpen = useRef(true);
  const [chatOpen, setChatOpen] = useState(() => {
    if (typeof window === "undefined") {
      preferredChatOpen.current = true;
      return !isOffline;
    }
    const isWide = window.matchMedia("(min-aspect-ratio: 1/1)").matches;
    let savedPreference: string | null = null;
    try {
      const preferenceKey = chatPreferenceKey(isWide);
      if (preferenceKey) {
        savedPreference = localStorage.getItem(preferenceKey);
      }
    } catch {
      // Storage can be unavailable in private browsing or embedded contexts.
    }
    preferredChatOpen.current = savedPreference !== "false";
    return shouldOpenChat(isOffline, savedPreference);
  });
  const wasOffline = useRef(isOffline);
  const userChangedChat = useRef(false);

  const toggleChat = useCallback(() => {
    userChangedChat.current = true;
    setChatOpen((prev) => {
      const next = !prev;
      preferredChatOpen.current = next;
      try {
        const isWide = window.matchMedia("(min-aspect-ratio: 1/1)").matches;
        const preferenceKey = chatPreferenceKey(isWide);
        if (preferenceKey) {
          localStorage.setItem(preferenceKey, String(next));
        }
      } catch {
        // The current view can still update when storage is unavailable.
      }
      return next;
    });
  }, []);

  // Auto-close when a live stream goes offline. When it becomes live again,
  // restore the intended state unless the viewer explicitly changed chat
  // during this visit. This covers false-offline startup results in any
  // orientation while preserving desktop preferences.
  useEffect(() => {
    const nextChatOpen = chatOpenAfterLivenessChange({
      isOffline,
      wasOffline: wasOffline.current,
      userChangedChat: userChangedChat.current,
      preferredChatOpen: preferredChatOpen.current,
      currentChatOpen: chatOpen,
    });
    if (nextChatOpen !== chatOpen) {
      setChatOpen(nextChatOpen);
    }
    wasOffline.current = isOffline;
  }, [isOffline, chatOpen]);

  return (
    <div className="flex h-full flex-col">
      {/* Sidebar layout (wide viewport) */}
      <div className="wide:flex wide:h-full wide:flex-col wide:gap-3 hidden">
        <div
          className={`z-0 flex min-h-0 flex-1 gap-4 transition-[margin] duration-300 ease-in-out ${chatOpen ? "wide:mr-90" : ""}`}
        >
          <div className="min-w-0 flex-1 overflow-y-auto">
            <VideoSection store={store} user={user} liveness={liveness} />
            {!theatre && (
              <StreamInfo
                store={store}
                user={user}
                liveness={liveness}
                chatOpen={chatOpen}
                onToggleChat={toggleChat}
                avatar={avatar}
              />
            )}
          </div>
        </div>

        <div
          className={`fixed top-12 right-0 bottom-0 z-20 flex w-90 flex-col overflow-hidden transition-transform duration-300 ease-in-out ${
            chatOpen ? "translate-x-0" : "translate-x-full"
          }`}
        >
          <ChatSidebar store={store} onClose={toggleChat} avatar={avatar} />
        </div>
      </div>

      {/* Stacked layout (portrait/tall) */}
      <div className="wide:hidden flex min-h-0 flex-1 flex-col">
        <VideoSection store={store} user={user} liveness={liveness} />

        <MobileStreamBar
          store={store}
          user={user}
          chatOpen={chatOpen}
          onToggleChat={toggleChat}
          avatar={avatar}
        />

        {!chatOpen && !isOffline && (
          <MobileStreamDetails store={store} user={user} avatar={avatar} />
        )}

        <div
          className={`min-h-0 flex-1 flex-col border-t ${chatOpen ? "flex" : "hidden"}`}
        >
          <ChatSidebar store={store} avatar={avatar} />
        </div>
      </div>
    </div>
  );
}

function MobileStreamBar({
  store,
  user,
  chatOpen,
  onToggleChat,
  avatar,
}: {
  store: LivestreamStore;
  user: string;
  chatOpen: boolean;
  onToggleChat: () => void;
  avatar?: string;
}) {
  const { t } = useTranslation("common");
  const state = useStore(
    store,
    useShallow((s) => ({
      livestream: s.livestream,
      viewers: s.viewers,
    })),
  );

  const author = state.livestream?.author;
  const record = state.livestream?.record;
  const title = record?.title || user;
  const authorLabel = author?.displayName || author?.handle || user;
  const node = useStreamplaceUrl();

  return (
    <div className="flex items-center gap-2 border-b border-(--color-border) bg-(--color-bg) px-3 py-2.5">
      <StreamAvatar
        avatar={avatar ?? author?.avatar}
        label={authorLabel}
        className="h-8 w-8 text-xs"
      />

      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <div className="truncate text-sm font-semibold">{title}</div>
          {state.viewers != null && (
            <span className="flex shrink-0 items-center gap-1 text-[11px] font-medium text-(--color-fg-muted)">
              <span className="h-1.5 w-1.5 rounded-full bg-(--color-accent)" />
              {t("watching-count", { count: state.viewers })}
            </span>
          )}
        </div>
        <div className="truncate text-xs text-(--color-fg-muted)">
          {author?.handle ? `@${author.handle}` : authorLabel}
        </div>
      </div>

      {chatOpen && (
        <Sheet>
          <SheetTrigger
            render={
              <button
                type="button"
                className="flex size-11 items-center justify-center rounded-lg text-(--color-fg-muted) transition-colors hover:bg-(--color-bg-overlay) hover:text-(--color-fg)"
                aria-label={t("stream-info")}
                title={t("stream-info")}
              >
                <Info className="h-4 w-4" />
              </button>
            }
          />
          <SheetContent side="bottom" className="rounded-t-xl">
            <SheetHeader>
              <SheetTitle>{title}</SheetTitle>
            </SheetHeader>
            <div className="px-4 pb-6">
              <MobileStreamDetailsContent
                store={store}
                user={user}
                avatar={avatar}
              />
            </div>
          </SheetContent>
        </Sheet>
      )}

      <CopyButton
        type="live"
        nodeBaseURL={node}
        variant="ghost"
        size="icon-touch"
        className="text-(--color-fg-muted) hover:bg-(--color-bg-overlay) hover:text-(--color-fg)"
      />

      <button
        type="button"
        onClick={onToggleChat}
        className="flex size-11 items-center justify-center rounded-lg text-(--color-fg-muted) transition-colors hover:bg-(--color-bg-overlay) hover:text-(--color-fg)"
        aria-label={chatOpen ? t("chat-close") : t("chat-open")}
        title={chatOpen ? t("chat-close") : t("chat-open")}
      >
        {chatOpen ? (
          <X className="h-4 w-4" />
        ) : (
          <MessageCircle className="h-4 w-4" />
        )}
      </button>
    </div>
  );
}

function MobileStreamDetails({
  store,
  user,
  avatar,
}: {
  store: LivestreamStore;
  user: string;
  avatar?: string;
}) {
  const { t } = useTranslation("common");

  return (
    <section className="min-h-0 flex-1 overflow-y-auto bg-(--color-bg) px-4 py-5">
      <div className="mx-auto w-full max-w-xl">
        <h2 className="font-display mb-4 text-lg font-semibold text-(--color-fg)">
          {t("stream-details")}
        </h2>
        <MobileStreamDetailsContent store={store} user={user} avatar={avatar} />
      </div>
    </section>
  );
}

function MobileStreamDetailsContent({
  store,
  user,
  avatar,
}: {
  store: LivestreamStore;
  user: string;
  avatar?: string;
}) {
  const { t } = useTranslation("common");
  const state = useStore(
    store,
    useShallow((s) => ({
      livestream: s.livestream,
    })),
  );

  const author = state.livestream?.author;
  const record = state.livestream?.record;
  const authorLabel = author?.displayName || author?.handle || user;
  const activity = activityLabel(record?.activity, t);
  const tags = record?.tags ?? [];
  const description = (
    record as { description?: string } | undefined
  )?.description?.trim();

  return (
    <div className="space-y-5">
      {(activity || tags.length > 0) && (
        <div className="flex flex-wrap items-center gap-1.5">
          {activity && (
            <span className="rounded-full border border-(--color-border) bg-(--color-bg-elevated) px-2 py-0.5 text-xs font-medium text-(--color-fg-muted)">
              {activity}
            </span>
          )}
          {tags.map((tag) => (
            <span
              key={tag}
              className="rounded-full border border-(--color-border) bg-(--color-bg-elevated) px-2 py-0.5 text-xs text-(--color-fg-subtle)"
            >
              {tag.startsWith("lang:") ? tag.slice(5).toUpperCase() : tag}
            </span>
          ))}
        </div>
      )}

      {description && (
        <p className="text-sm leading-relaxed whitespace-pre-wrap text-(--color-fg)">
          {description}
        </p>
      )}

      <div className="flex items-center gap-3 border-t border-(--color-border) pt-4">
        <StreamAvatar
          avatar={avatar ?? author?.avatar}
          label={authorLabel}
          className="h-11 w-11"
        />
        <div className="min-w-0 flex-1">
          <div className="truncate font-medium text-(--color-fg)">
            {authorLabel}
          </div>
          {author?.handle && (
            <div className="truncate text-sm text-(--color-fg-muted)">
              @{author.handle}
            </div>
          )}
        </div>
        {author?.handle && (
          <a
            href={`https://bsky.app/profile/${author.handle}`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex size-11 items-center justify-center rounded-lg text-(--color-fg-muted) transition-colors hover:bg-(--color-bg-overlay) hover:text-(--color-fg)"
            aria-label={t("view-profile")}
            title={t("view-profile")}
          >
            <ExternalLink className="h-4 w-4" />
          </a>
        )}
      </div>

      <a
        href={`/chat-popout/${encodeURIComponent(author?.handle || user)}`}
        target="_blank"
        rel="noopener noreferrer"
        className="inline-flex min-h-11 items-center gap-2 rounded-lg px-3 text-sm font-medium text-(--color-fg-muted) transition-colors hover:bg-(--color-bg-overlay) hover:text-(--color-fg)"
      >
        <ExternalLink className="h-4 w-4" />
        {t("chat-pop-out")}
      </a>
    </div>
  );
}
