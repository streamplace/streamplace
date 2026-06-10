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
    <div className="flex flex-col flex-1 min-h-0 border-l">
      <SidebarHeader>
        <div className="flex items-center gap-3 py-1">
          <img
            src={author?.avatar ?? undefined}
            alt=""
            className="w-8 h-8 rounded-full bg-[var(--color-bg)] flex-shrink-0"
            onError={(e) => {
              (e.currentTarget as HTMLImageElement).style.display = "none";
            }}
          />
          <div className="min-w-0 flex-1">
            <div className="text-sm font-medium truncate">
              {author?.displayName || author?.handle || t("streamer-fallback")}
            </div>
            {viewers && (
              <div className="text-xs text-[var(--color-fg-subtle)]">
                {t("watching-count", { count: state.viewers ?? 0 })}
              </div>
            )}
          </div>

          <div className="flex items-center gap-1">
            <a
              href={`/chat-popout/${encodeURIComponent(author?.handle || "")}`}
              target="_blank"
              rel="noopener noreferrer"
              className="p-1 rounded hover:bg-[var(--color-bg-overlay)] text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
              title={t("chat-pop-out")}
            >
              <ExternalLink className="w-3.5 h-3.5" />
            </a>

            {onClose && (
              <button
                type="button"
                onClick={onClose}
                className="p-1 rounded hover:bg-[var(--color-bg-overlay)] text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors flex-shrink-0"
                aria-label={t("chat-close")}
              >
                <svg
                  className="w-4 h-4"
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

      <SidebarContent className="p-0! overflow-hidden">
        <ChatPanel store={store} />
      </SidebarContent>

      <SidebarFooter>
        <ChatInput store={store} />
      </SidebarFooter>
    </div>
  );
}
