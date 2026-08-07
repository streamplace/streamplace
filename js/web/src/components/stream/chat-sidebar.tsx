import type { LivestreamStore } from "@streamplace/core";
import { ExternalLink } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import { formatViewers } from "../../lib/format";
import { SidebarContent, SidebarFooter, SidebarHeader } from "../ui/sidebar";
import { ChatInput } from "./chat-input";
import { ChatPanel } from "./chat-panel";
import { StreamNotifications } from "./stream-notifications";

export function ChatSidebar({
  store,
  onClose,
}: {
  store: LivestreamStore;
  onClose?: () => void;
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
  const viewers = formatViewers(state.viewers);

  return (
    <div className="flex min-h-0 flex-1 flex-col border-l">
      <SidebarHeader className="wide:flex hidden">
        <div className="flex items-center gap-3 py-1">
          <img
            src={author?.avatar ?? undefined}
            alt=""
            className="h-8 w-8 shrink-0 rounded-full bg-(--color-bg)"
            onError={(e) => {
              (e.currentTarget as HTMLImageElement).style.display = "none";
            }}
          />
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm font-medium">
              {author?.displayName || author?.handle || t("streamer-fallback")}
            </div>
            {viewers && (
              <div className="text-xs text-(--color-fg-subtle)">
                {t("watching-count", { count: state.viewers ?? 0 })}
              </div>
            )}
          </div>

          <div className="flex items-center gap-1">
            <a
              href={`/chat-popout/${encodeURIComponent(author?.handle || "")}`}
              target="_blank"
              rel="noopener noreferrer"
              className="rounded p-1 text-(--color-fg-muted) transition-colors hover:bg-(--color-bg-overlay) hover:text-(--color-fg)"
              title={t("chat-pop-out")}
            >
              <ExternalLink className="h-3.5 w-3.5" />
            </a>

            {onClose && (
              <button
                type="button"
                onClick={onClose}
                className="shrink-0 rounded p-1 text-(--color-fg-muted) transition-colors hover:bg-(--color-bg-overlay) hover:text-(--color-fg)"
                aria-label={t("chat-close")}
              >
                <svg
                  className="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M9 5l7 7-7 7"
                  />
                </svg>
              </button>
            )}
          </div>
        </div>
      </SidebarHeader>

      <StreamNotifications store={store} />

      <SidebarContent className="overflow-hidden p-0!">
        <ChatPanel store={store} />
      </SidebarContent>

      <SidebarFooter>
        <ChatInput store={store} />
      </SidebarFooter>
    </div>
  );
}
