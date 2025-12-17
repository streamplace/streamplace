import { TriggerRef, useRootContext } from "@rn-primitives/dropdown-menu";
import { forwardRef, useEffect, useRef, useState } from "react";
import { gap, mr, w } from "../../lib/theme/atoms";
import { usePlayerStore } from "../../player-store";
import {
  useCreateBlockRecord,
  useCreateHideChatRecord,
} from "../../streamplace-store/block";
import { useCanModerate } from "../../streamplace-store/moderation";
import { usePDSAgent } from "../../streamplace-store/xrpc";

import { Linking } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import {
  useDeleteChatMessage,
  useLivestreamStore,
} from "../../livestream-store";
import { useStreamplaceStore } from "../../streamplace-store";
import { formatHandle, formatHandleWithAt } from "../../utils/format-handle";
import {
  atoms,
  DropdownMenu,
  DropdownMenuGroup,
  DropdownMenuItem,
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

  let agent = usePDSAgent();
  let [messageRemoved, setMessageRemoved] = useState(false);
  let { createBlock, isLoading: isBlockLoading } = useCreateBlockRecord();
  let { createHideChat, isLoading: isHideLoading } = useCreateHideChatRecord();

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

  if (!agent?.did) {
    <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
      <Text>Log in to submit mod actions</Text>
    </View>;
  }

  const cleanup = () => {
    setModMessage(null);
  };

  useEffect(() => {
    if (message) {
      setMessageRemoved(false);
      triggerRef.current?.open();
    } else {
      triggerRef.current?.close();
    }
  }, [message]);

  // Can show moderation actions if user can hide or ban
  const canModerate = modPermissions.canHide || modPermissions.canBan;

  return (
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
          <>
            <DropdownMenuGroup>
              <DropdownMenuItem>
                <View
                  style={[
                    layout.flex.column,
                    mr[5],
                    { gap: 6, maxWidth: "100%" },
                  ]}
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

            {canModerate && (
              <DropdownMenuGroup title={`Moderation actions`}>
                {modPermissions.canHide && (
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
                      color={
                        isHideLoading || messageRemoved
                          ? "muted"
                          : "destructive"
                      }
                    >
                      {isHideLoading
                        ? "Removing..."
                        : messageRemoved
                          ? "Message removed"
                          : "Remove this message"}
                    </Text>
                  </DropdownMenuItem>
                )}
                {modPermissions.canBan && (
                  <DropdownMenuItem
                    disabled={
                      message.author.did === agent?.did || isBlockLoading
                    }
                    onPress={() => {
                      createBlock(message.author.did, streamerDID ?? undefined)
                        .then((r) => console.log(r))
                        .catch((e) => console.error(e));
                    }}
                  >
                    {message.author.did === agent?.did ? (
                      <Text color="muted">
                        Block yourself (you can't block yourself)
                      </Text>
                    ) : (
                      <Text color="destructive">
                        {isBlockLoading
                          ? "Blocking..."
                          : `Block user ${formatHandleWithAt(message.author)} from this channel`}
                      </Text>
                    )}
                  </DropdownMenuItem>
                )}
              </DropdownMenuGroup>
            )}

            <DropdownMenuGroup title={`User actions`}>
              <DropdownMenuItem
                onPress={() => {
                  Linking.openURL(
                    `https://${BSKY_FRONTEND_DOMAIN}/profile/${formatHandle(message.author)}`,
                  );
                }}
              >
                <Text color="primary">View user on {BSKY_FRONTEND_DOMAIN}</Text>
              </DropdownMenuItem>
              {message.author.did === agent?.did && (
                <DeleteButton
                  message={message}
                  deleteChatMessage={deleteChatMessage}
                />
              )}
              {message.author.did !== agent?.did && (
                <ReportButton
                  message={message}
                  setReportModalOpen={setReportModalOpen}
                  setReportSubject={setReportSubject}
                />
              )}
            </DropdownMenuGroup>
          </>
        )}
      </ResponsiveDropdownMenuContent>
    </DropdownMenu>
  );
});

enum DeleteState {
  None,
  Confirmed,
  Deleting,
}

export function DeleteButton({
  message,
  deleteChatMessage,
}: {
  message: ChatMessageViewHydrated;
  deleteChatMessage: (uri: string) => Promise<any>;
}) {
  const [confirming, setConfirming] = useState<DeleteState>(DeleteState.None);
  const { onOpenChange } = useRootContext();
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
}: {
  message: ChatMessageViewHydrated;
  setReportModalOpen: (open: boolean) => void;
  setReportSubject: (subject: any) => void;
}) {
  const { onOpenChange } = useRootContext();
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
