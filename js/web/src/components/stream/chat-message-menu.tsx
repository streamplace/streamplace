import type { UseCanModerateResult } from "@/hooks/use-can-moderate";
import { useModerationActions } from "@/hooks/use-moderation-actions";
import { useToast } from "@/hooks/use-toast";
import { useSession } from "@/lib/session";
import type { LivestreamStore } from "@streamplace/core";
import { reduceChat } from "@streamplace/core";
import { EyeOff, MoreHorizontal, Pin, Shield, Trash2 } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { ChatMessageViewHydrated } from "streamplace";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu";

const PIN_DURATIONS_MINUTES = [5, 10, 15, 30, 60];

interface ChatMessageMenuProps {
  store: LivestreamStore;
  message: ChatMessageViewHydrated;
  streamerDid: string | undefined;
  isOwn: boolean;
  moderation: UseCanModerateResult;
}

/**
 * Hover menu on a chat message: pin/hide/block for the streamer and
 * delegated moderators, plus delete for the viewer's own messages.
 */
export function ChatMessageMenu({
  store,
  message,
  streamerDid,
  isOwn,
  moderation,
}: ChatMessageMenuProps) {
  const { t } = useTranslation("common");
  const { pdsAgent, did } = useSession();
  const toast = useToast();
  const { blockUser, hideMessage, pinMessage } = useModerationActions();
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const deleteTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (deleteTimer.current) clearTimeout(deleteTimer.current);
    },
    [],
  );

  const authorDid = message.author.did;
  const canPin = moderation.canPin && authorDid !== streamerDid;
  const canHide = moderation.canHide && authorDid !== streamerDid;
  const canBlock =
    moderation.canBan &&
    !!did &&
    authorDid !== did &&
    authorDid !== streamerDid;

  const fail = useCallback(
    (fallback: string, e: unknown) => {
      console.error(e);
      toast.show(
        t("moderation-error-title"),
        e instanceof Error ? e.message : fallback,
        { duration: 5000 },
      );
    },
    [t, toast],
  );

  const handleHide = useCallback(async () => {
    if (!streamerDid) return;
    try {
      await hideMessage(message.uri, streamerDid);
      // Show the message as gone right away; the chat gate arriving over
      // websocket removes it for everyone else.
      store.setState((s) => reduceChat(s, [], [], [message.uri]));
    } catch (e) {
      fail(t("moderation-hide-failed"), e);
    }
  }, [fail, hideMessage, message.uri, store, streamerDid, t]);

  const handleBlock = useCallback(async () => {
    if (!streamerDid) return;
    try {
      await blockUser(authorDid, streamerDid);
      toast.show(
        t("moderation-user-blocked", { handle: message.author.handle }),
        "",
        { duration: 3000 },
      );
    } catch (e) {
      fail(t("moderation-block-failed"), e);
    }
  }, [
    authorDid,
    blockUser,
    fail,
    message.author.handle,
    streamerDid,
    t,
    toast,
  ]);

  const handlePin = useCallback(
    async (expiresAt?: string) => {
      if (!streamerDid) return;
      try {
        await pinMessage(message.uri, streamerDid, expiresAt);
        toast.show(t("moderation-message-pinned"), "", { duration: 3000 });
      } catch (e) {
        fail(t("moderation-pin-failed"), e);
      }
    },
    [fail, message.uri, pinMessage, streamerDid, t, toast],
  );

  const handleDeleteOwn = useCallback(async () => {
    if (!pdsAgent?.did) return;
    const rkey = message.uri.split("/").pop();
    if (!rkey) return;
    try {
      await pdsAgent.com.atproto.repo.deleteRecord({
        repo: pdsAgent.did as any,
        collection: "place.stream.chat.message",
        rkey,
      });
    } catch (e) {
      fail(t("moderation-delete-failed"), e);
    }
  }, [fail, message.uri, pdsAgent, t]);

  const handleDeleteClick = useCallback(() => {
    // Two-step confirm; reset if the user reopens the menu later.
    if (!confirmingDelete) {
      setConfirmingDelete(true);
      deleteTimer.current = setTimeout(() => setConfirmingDelete(false), 4000);
      return;
    }
    if (deleteTimer.current) clearTimeout(deleteTimer.current);
    setConfirmingDelete(false);
    handleDeleteOwn();
  }, [confirmingDelete, handleDeleteOwn]);

  if (!did) return null;
  if (!isOwn && !canPin && !canHide && !canBlock) return null;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="rounded border border-(--color-border) bg-(--color-bg-elevated) p-1 text-(--color-fg-muted) shadow-sm transition-colors hover:bg-(--color-bg-overlay) hover:text-(--color-fg)"
        aria-label={t("chat-moderation-menu")}
      >
        <MoreHorizontal className="h-3.5 w-3.5" />
      </DropdownMenuTrigger>
      <DropdownMenuContent side="top" align="end" className="min-w-48">
        {canPin && (
          <DropdownMenuSub>
            <DropdownMenuSubTrigger>
              <Pin /> {t("chat-pin-message")}
            </DropdownMenuSubTrigger>
            <DropdownMenuSubContent>
              <DropdownMenuGroup>
                <DropdownMenuItem onClick={() => handlePin()}>
                  {t("chat-pin-until-stream-end")}
                </DropdownMenuItem>
                {PIN_DURATIONS_MINUTES.map((minutes) => (
                  <DropdownMenuItem
                    key={minutes}
                    onClick={() =>
                      handlePin(
                        new Date(
                          Date.now() + minutes * 60 * 1000,
                        ).toISOString(),
                      )
                    }
                  >
                    {t("chat-pin-duration-minutes", { count: minutes })}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuGroup>
            </DropdownMenuSubContent>
          </DropdownMenuSub>
        )}
        {canHide && (
          <DropdownMenuItem variant="destructive" onClick={handleHide}>
            <EyeOff /> {t("chat-hide-message")}
          </DropdownMenuItem>
        )}
        {canBlock && (
          <DropdownMenuItem variant="destructive" onClick={handleBlock}>
            <Shield /> {t("chat-block-user")}
          </DropdownMenuItem>
        )}
        {isOwn && (
          <DropdownMenuItem variant="destructive" onClick={handleDeleteClick}>
            <Trash2 />
            {confirmingDelete
              ? t("chat-delete-confirm")
              : t("chat-delete-message")}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
