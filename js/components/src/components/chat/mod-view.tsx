import { TriggerRef, useRootContext } from "@rn-primitives/dropdown-menu";
import { forwardRef, useEffect, useRef, useState } from "react";
import { gap, mr, w } from "../../lib/theme/atoms";
import { usePlayerStore } from "../../player-store";
import {
  useCreateBlockRecord,
  useCreateHideChatRecord,
} from "../../streamplace-store/block";
import {
  ModerationPermissions,
  useCanModerate,
} from "../../streamplace-store/moderation";
import { usePDSAgent } from "../../streamplace-store/xrpc";

import { Linking } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import {
  useDeleteChatMessage,
  useLivestream,
  useLivestreamStore,
  usePinChatMessage,
} from "../../livestream-store";
import { useStreamplaceStore } from "../../streamplace-store";
import { formatHandle, formatHandleWithAt } from "../../utils/format-handle";
import {
  atoms,
  DropdownMenu,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
  layout,
  ResponsiveDropdownMenuContent,
  Text,
  useToast,
  View,
} from "../ui";

const BSKY_FRONTEND_DOMAIN = "bsky.app";

type ModViewProps = {
  onClose?: () => void;
  // onDeleteMessage?: (msg: ChatMessageViewHydrated) => void;
  // onBanUser?: (userHandle: string) => void;
};

export type ModViewRef = {
  open: () => void;
  close: () => void;
};

export const ModView = forwardRef<ModViewRef, ModViewProps>(() => {
  const triggerRef = useRef<TriggerRef>(null);
  const message = usePlayerStore((state) => state.modMessage);
  const toast = useToast();

  let agent = usePDSAgent();
  let [messageRemoved, setMessageRemoved] = useState(false);
  let { createBlock, isLoading: isBlockLoading } = useCreateBlockRecord();
  let { createHideChat, isLoading: isHideLoading } = useCreateHideChatRecord();
  const pinChatMessage = usePinChatMessage();
  const livestream = useLivestream();

  const setReportModalOpen = usePlayerStore((x) => x.setReportModalOpen);
  const setReportSubject = usePlayerStore((x) => x.setReportSubject);
  const setModMessage = usePlayerStore((x) => x.setModMessage);
  const deleteChatMessage = useDeleteChatMessage();

  // Get the streamer's DID from the livestream profile
  const streamerDID = useLivestreamStore((x) => x.profile?.did);
  // Check moderation permissions for the current user on this streamer's channel
  const modPermissions = useCanModerate(streamerDID);

  // get the channel did
  const channelId = usePlayerStore((state) => state.src);
  // get the logged in user's identity
  const handle = useStreamplaceStore((state) => state.handle);

  const cleanup = () => {
    setModMessage(null);
  };

  // Effect must be called unconditionally (before any early returns)
  useEffect(() => {
    if (message) {
      setMessageRemoved(false);
      triggerRef.current?.open();
    } else {
      triggerRef.current?.close();
    }
  }, [message]);

  // Check if any moderation actions are actually available for this message
  // This must match the individual action checks inside the DropdownMenuGroup
  const hasAvailableActions = !!(
    message &&
    agent?.did &&
    ((modPermissions.canHide && message.author.did !== streamerDID) ||
      (modPermissions.canPin && message.author.did !== streamerDID) ||
      modPermissions.canBan)
  );

  return (
    <>
      <DropdownMenu
        style={[layout.flex.row, layout.flex.alignCenter, gap.all[2], w[80]]}
        onOpenChange={(isOpen) => {
          if (!isOpen) {
            cleanup();
          }
        }}
      >
        <DropdownMenuTrigger ref={triggerRef}>
          {/* Hidden trigger */}
          <View />
        </DropdownMenuTrigger>
        <ResponsiveDropdownMenuContent>
          {message && (
            <ModViewContent
              message={message}
              modPermissions={modPermissions}
              agent={agent}
              streamerDID={streamerDID}
              livestreamUri={livestream?.uri}
              hasAvailableActions={hasAvailableActions}
              isHideLoading={isHideLoading}
              isBlockLoading={isBlockLoading}
              messageRemoved={messageRemoved}
              setMessageRemoved={setMessageRemoved}
              createHideChat={createHideChat}
              createBlock={createBlock}
              pinChatMessage={pinChatMessage}
              toast={toast}
              setReportModalOpen={setReportModalOpen}
              setReportSubject={setReportSubject}
              deleteChatMessage={deleteChatMessage}
            />
          )}
        </ResponsiveDropdownMenuContent>
      </DropdownMenu>
    </>
  );
});

interface ModViewContentProps {
  message: ChatMessageViewHydrated;
  modPermissions: ModerationPermissions;
  agent: ReturnType<typeof usePDSAgent>;
  streamerDID?: string;
  livestreamUri?: string;
  hasAvailableActions: boolean;
  isHideLoading: boolean;
  isBlockLoading: boolean;
  messageRemoved: boolean;
  setMessageRemoved: (removed: boolean) => void;
  createHideChat: (uri: string, streamerDID?: string) => Promise<any>;
  createBlock: (did: string, streamerDID?: string) => Promise<any>;
  pinChatMessage: (
    messageUri: string,
    streamerDID: string,
    options?: { expiresAt?: string; duration?: string; livestream?: string },
  ) => Promise<any>;
  toast: ReturnType<typeof useToast>;
  setReportModalOpen: (open: boolean) => void;
  setReportSubject: (subject: any) => void;
  deleteChatMessage: (uri: string) => Promise<any>;
}

function ModViewContent({
  message,
  modPermissions,
  agent,
  streamerDID,
  livestreamUri,
  hasAvailableActions,
  isHideLoading,
  isBlockLoading,
  messageRemoved,
  setMessageRemoved,
  createHideChat,
  createBlock,
  pinChatMessage,
  toast,
  setReportModalOpen,
  setReportSubject,
  deleteChatMessage,
}: ModViewContentProps) {
  const { onOpenChange } = useRootContext();

  return (
    <>
      <DropdownMenuGroup key="message-display">
        <DropdownMenuItem>
          <View
            style={[layout.flex.column, mr[5], { gap: 6, maxWidth: "100%" }]}
          >
            <Text
              style={{
                fontVariant: ["tabular-nums"],
                color: atoms.colors.gray[300],
              }}
            >
              {new Date(message.record.createdAt).toLocaleTimeString([], {
                hour: "2-digit",
                minute: "2-digit",
                hour12: false,
              })}{" "}
              {formatHandleWithAt(message.author)}: {message.record.text}
            </Text>
          </View>
        </DropdownMenuItem>
      </DropdownMenuGroup>

      {hasAvailableActions && (
        <DropdownMenuGroup
          key="moderation-actions"
          title={`Moderation actions`}
        >
          {modPermissions.canHide && message.author.did !== streamerDID && (
            <DropdownMenuItem
              disabled={isHideLoading || messageRemoved}
              onPress={() => {
                if (isHideLoading || messageRemoved) return;
                createHideChat(message.uri, streamerDID ?? undefined)
                  .then((r) => setMessageRemoved(true))
                  .catch((e) => console.error(e));
              }}
            >
              <Text
                color={isHideLoading || messageRemoved ? "muted" : "warning"}
              >
                {isHideLoading
                  ? "Hiding..."
                  : messageRemoved
                    ? "Message hidden"
                    : "Hide this message"}
              </Text>
            </DropdownMenuItem>
          )}
          {modPermissions.canPin && (
            <DropdownMenuGroup key="pin-actions">
              <DropdownMenuSub>
                <DropdownMenuSubTrigger
                  subMenuTitle="Pin message"
                  style={{ padding: 0, margin: 0 }}
                >
                  <Text color="primary">Pin this message</Text>
                </DropdownMenuSubTrigger>
                <DropdownMenuSubContent>
                  <DropdownMenuGroup title="Pin duration">
                    <DropdownMenuItem
                      onPress={() => {
                        if (!streamerDID) return;
                        pinChatMessage(message.uri, streamerDID, {
                          livestream: livestreamUri,
                        })
                          .then(() => {
                            toast.show("Comment pinned", "", { duration: 3 });
                            onOpenChange?.(false);
                          })
                          .catch((e) => {
                            toast.show(
                              "Error pinning comment",
                              e instanceof Error ? e.message : "Failed to pin",
                              { duration: 5 },
                            );
                          });
                      }}
                    >
                      <Text color="primary">Until stream end</Text>
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      onPress={() => {
                        if (!streamerDID) return;
                        pinChatMessage(message.uri, streamerDID)
                          .then(() => {
                            toast.show("Comment pinned", "", { duration: 3 });
                            onOpenChange?.(false);
                          })
                          .catch((e) => {
                            toast.show(
                              "Error pinning comment",
                              e instanceof Error ? e.message : "Failed to pin",
                              { duration: 5 },
                            );
                          });
                      }}
                    >
                      <Text color="primary">Forever</Text>
                    </DropdownMenuItem>
                    {[5, 10, 15, 30, 60].map((minutes) => (
                      <DropdownMenuItem
                        key={minutes}
                        onPress={() => {
                          if (!streamerDID) return;
                          const expiresAt = new Date(
                            Date.now() + minutes * 60 * 1000,
                          );
                          pinChatMessage(message.uri, streamerDID, {
                            expiresAt: expiresAt.toISOString(),
                          })
                            .then(() => {
                              toast.show("Comment pinned", "", { duration: 3 });
                              onOpenChange?.(false);
                            })
                            .catch((e) => {
                              toast.show(
                                "Error pinning comment",
                                e instanceof Error
                                  ? e.message
                                  : "Failed to pin",
                                { duration: 5 },
                              );
                            });
                        }}
                      >
                        <Text color="primary">
                          {minutes < 60 ? `${minutes} min` : "1 hour"}
                        </Text>
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuGroup>
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            </DropdownMenuGroup>
          )}
          {modPermissions.canBan &&
            agent?.did &&
            message.author.did !== agent.did &&
            message.author.did !== streamerDID && (
              <DropdownMenuItem
                disabled={isBlockLoading}
                onPress={() => {
                  if (isBlockLoading) return;
                  createBlock(message.author.did, streamerDID ?? undefined)
                    .then((r) => {
                      toast.show(
                        "User blocked",
                        `${formatHandleWithAt(message.author)} has been blocked from this channel.`,
                        { duration: 3 },
                      );
                      onOpenChange?.(false);
                    })
                    .catch((e) => {
                      console.error(e);
                      toast.show(
                        "Error blocking user",
                        e instanceof Error ? e.message : "Failed to block user",
                        { duration: 5 },
                      );
                    });
                }}
              >
                <Text color="destructive">
                  {isBlockLoading
                    ? "Blocking..."
                    : `Block user ${formatHandleWithAt(message.author)} from this channel`}
                </Text>
              </DropdownMenuItem>
            )}
        </DropdownMenuGroup>
      )}

      <DropdownMenuGroup key="user-actions" title={`User actions`}>
        <DropdownMenuItem
          onPress={() => {
            Linking.openURL(
              `https://${BSKY_FRONTEND_DOMAIN}/profile/${formatHandle(message.author)}`,
            );
          }}
        >
          <Text color="primary">View user on {BSKY_FRONTEND_DOMAIN}</Text>
        </DropdownMenuItem>
        {agent?.did && message.author.did === agent.did && (
          <DeleteButton
            message={message}
            deleteChatMessage={deleteChatMessage}
            onOpenChange={onOpenChange}
          />
        )}
        {(!agent?.did || message.author.did !== agent.did) && (
          <ReportButton
            message={message}
            setReportModalOpen={setReportModalOpen}
            setReportSubject={setReportSubject}
            onOpenChange={onOpenChange}
          />
        )}
      </DropdownMenuGroup>
    </>
  );
}

enum DeleteState {
  None,
  Confirmed,
  Deleting,
}

export function DeleteButton({
  message,
  deleteChatMessage,
  onOpenChange,
}: {
  message: ChatMessageViewHydrated;
  deleteChatMessage: (uri: string) => Promise<any>;
  onOpenChange?: (open: boolean) => void;
}) {
  const [confirming, setConfirming] = useState<DeleteState>(DeleteState.None);
  const toast = useToast();
  return (
    <DropdownMenuItem
      closeOnPress={false}
      onPress={() => {
        if (!message) return;
        if (!confirming) {
          setConfirming(DeleteState.Confirmed);
          return;
        }
        if (confirming === DeleteState.Confirmed) {
          setConfirming(DeleteState.Deleting);
        }
        deleteChatMessage(message.uri)
          .then(() => {
            // wait ~a second before resetting state to allow deletion to take effect
            setTimeout(() => setConfirming(DeleteState.None), 1000);
            onOpenChange?.(false);
          })
          .catch((e) => {
            toast.show("Couldn't delete the message", e);
            setConfirming(DeleteState.None);
          });
      }}
    >
      <Text color="destructive">
        {confirming === DeleteState.Confirmed
          ? "Are you sure? Click again to confirm."
          : confirming === DeleteState.Deleting
            ? "Deleting..."
            : "Delete message"}
      </Text>
    </DropdownMenuItem>
  );
}

export function ReportButton({
  message,
  setReportModalOpen,
  setReportSubject,
  onOpenChange,
}: {
  message: ChatMessageViewHydrated;
  setReportModalOpen: (open: boolean) => void;
  setReportSubject: (subject: any) => void;
  onOpenChange?: (open: boolean) => void;
}) {
  return (
    <DropdownMenuItem
      onPress={() => {
        if (!message) return;
        onOpenChange?.(false);
        setReportModalOpen(true);
        setReportSubject({
          $type: "com.atproto.repo.strongRef",
          uri: message.uri,
          cid: message.cid,
          context: message,
        });
      }}
    >
      <Text color="warning">Report chat...</Text>
    </DropdownMenuItem>
  );
}
