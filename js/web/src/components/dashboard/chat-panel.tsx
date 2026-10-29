import { useToast } from "@/hooks/use-toast";
import type { LivestreamStore } from "@streamplace/core";
import { useNavigate } from "@tanstack/react-router";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import { ChatInput } from "../stream/chat-input";
import { ChatPanel as ChatMessages } from "../stream/chat-panel";

/**
 * Dashboard chat panel with message count and messages/min rate.
 * Wraps the existing ChatPanel and ChatInput components.
 */
export function ChatPanelWidget({ store }: { store: LivestreamStore }) {
  const { t } = useTranslation("common");
  const toast = useToast();
  const navigate = useNavigate();
  const state = useStore(
    store,
    useShallow((s) => ({
      chat: s.chat,
      websocketConnected: s.websocketConnected,
      livestream: s.livestream,
    })),
  );

  const isConnected = state.websocketConnected;
  const isLive = !!state.livestream;

  // Messages per minute calculation
  const messagesPerMinute = useMemo(() => {
    if (!state.chat) return 0;
    const oneMinuteAgo = Date.now() - 60 * 1000;
    return state.chat.filter((msg) => {
      try {
        return new Date(msg.indexedAt).getTime() > oneMinuteAgo;
      } catch {
        return false;
      }
    }).length;
  }, [state.chat]);

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-b-lg border border-(--color-border) bg-(--color-bg-elevated)">
      {/* Messages */}
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <ChatMessages store={store} />
      </div>

      {/* Input */}
      <div className="shrink-0 border-t border-(--color-border)">
        <ChatInput
          store={store}
          onTeleportCreated={(_uri, targetDID) => {
            toast.show("Teleport started", "Visit the receiving livestream.", {
              variant: "success",
              actionLabel: "Go to livestream",
              onAction: () => {
                void navigate({
                  to: "/$user",
                  params: { user: targetDID },
                });
              },
            });
          }}
        />
      </div>
    </div>
  );
}
