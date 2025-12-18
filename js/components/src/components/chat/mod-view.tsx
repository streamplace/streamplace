import { TriggerRef, useRootContext } from "@rn-primitives/dropdown-menu";
import { forwardRef, useEffect, useRef, useState } from "react";
import { gap, mr, w } from "../../lib/theme/atoms";
import { usePlayerStore } from "../../player-store";
import {
  useCreateBlockRecord,
  useCreateHideChatRecord,
  useUpdateLivestreamRecord,
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
} from "../../livestream-store";
import { useStreamplaceStore } from "../../streamplace-store";
import { formatHandle, formatHandleWithAt } from "../../utils/format-handle";
import {
  atoms,
  Button,
  DialogFooter,
  DropdownMenu,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
  layout,
  ResponsiveDialog,
  ResponsiveDropdownMenuContent,
  Text,
  Textarea,
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
  let { updateLivestream, isLoading: isUpdateTitleLoading } =
    useUpdateLivestreamRecord();
  const livestream = useLivestream();
  const [showUpdateTitleDialog, setShowUpdateTitleDialog] = useState(false);

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

  // Early return AFTER all hooks have been called
  if (!agent?.did) {
    return (
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
        <Text>Log in to submit mod actions</Text>
      </View>
    );
  }

  // Can show moderation actions if user can hide, ban, or manage livestream
  const canModerate =
    modPermissions.canHide ||
    modPermissions.canBan ||
    modPermissions.canManageLivestream;

  // Check if any moderation actions are actually available for this message
  // This must match the individual action checks inside the DropdownMenuGroup
  const hasAvailableActions = !!(
    message &&
    agent?.did &&
    ((modPermissions.canHide && message.author.did !== streamerDID) ||
      (modPermissions.canBan &&
        message.author.did !== agent.did &&
        message.author.did !== streamerDID))
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
              hasAvailableActions={hasAvailableActions}
              isHideLoading={isHideLoading}
              isBlockLoading={isBlockLoading}
              messageRemoved={messageRemoved}
              setMessageRemoved={setMessageRemoved}
              createHideChat={createHideChat}
              createBlock={createBlock}
              toast={toast}
              setShowUpdateTitleDialog={setShowUpdateTitleDialog}
              isUpdateTitleLoading={isUpdateTitleLoading}
              livestream={livestream}
              setReportModalOpen={setReportModalOpen}
              setReportSubject={setReportSubject}
              deleteChatMessage={deleteChatMessage}
            />
          )}
        </ResponsiveDropdownMenuContent>
      </DropdownMenu>

      {/* Update Stream Title Dialog - rendered outside dropdown */}
      {showUpdateTitleDialog && (
        <UpdateStreamTitleDialog
          livestream={livestream}
          streamerDID={streamerDID}
          updateLivestream={updateLivestream}
          isLoading={isUpdateTitleLoading}
          onClose={() => setShowUpdateTitleDialog(false)}
        />
      )}
    </>
  );
});

interface ModViewContentProps {
  message: ChatMessageViewHydrated;
  modPermissions: ModerationPermissions;
  agent: ReturnType<typeof usePDSAgent>;
  streamerDID?: string;
  hasAvailableActions: boolean;
  isHideLoading: boolean;
  isBlockLoading: boolean;
  messageRemoved: boolean;
  setMessageRemoved: (removed: boolean) => void;
  createHideChat: (uri: string, streamerDID?: string) => Promise<any>;
  createBlock: (did: string, streamerDID?: string) => Promise<any>;
  toast: ReturnType<typeof useToast>;
  setShowUpdateTitleDialog: (show: boolean) => void;
  isUpdateTitleLoading: boolean;
  livestream: any;
  setReportModalOpen: (open: boolean) => void;
  setReportSubject: (subject: any) => void;
  deleteChatMessage: (uri: string) => Promise<any>;
}

function ModViewContent({
  message,
  modPermissions,
  agent,
  streamerDID,
  hasAvailableActions,
  isHideLoading,
  isBlockLoading,
  messageRemoved,
  setMessageRemoved,
  createHideChat,
  createBlock,
  toast,
  setShowUpdateTitleDialog,
  isUpdateTitleLoading,
  livestream,
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

      {modPermissions.canManageLivestream && (
        <DropdownMenuGroup key="stream-actions" title={`Stream actions`}>
          <DropdownMenuItem
            onPress={() => {
              setShowUpdateTitleDialog(true);
            }}
            disabled={isUpdateTitleLoading || !livestream}
          >
            <Text
              color={isUpdateTitleLoading || !livestream ? "muted" : "primary"}
            >
              {isUpdateTitleLoading ? "Updating..." : "Update stream title"}
            </Text>
          </DropdownMenuItem>
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
        {message.author.did === agent?.did && (
          <DeleteButton
            message={message}
            deleteChatMessage={deleteChatMessage}
            onOpenChange={onOpenChange}
          />
        )}
        {message.author.did !== agent?.did && (
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

interface UpdateStreamTitleDialogProps {
  livestream: any;
  streamerDID?: string;
  updateLivestream: (
    livestreamUri: string,
    title: string,
    streamerDID?: string,
  ) => Promise<any>;
  isLoading: boolean;
  onClose: () => void;
}

function UpdateStreamTitleDialog({
  livestream,
  streamerDID,
  updateLivestream,
  isLoading,
  onClose,
}: UpdateStreamTitleDialogProps) {
  const [title, setTitle] = useState(livestream?.record?.title || "");
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  useEffect(() => {
    if (livestream?.record?.title) {
      setTitle(livestream.record.title);
    }
  }, [livestream?.record?.title]);

  const handleUpdate = async () => {
    setError(null);

    if (!title.trim()) {
      setError("Please enter a stream title");
      return;
    }

    if (!livestream?.uri) {
      setError("No livestream found");
      return;
    }

    try {
      await updateLivestream(livestream.uri, title.trim(), streamerDID);
      toast.show(
        "Stream title updated",
        "The stream title has been successfully updated.",
        { duration: 3 },
      );
      onClose();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to update stream title",
      );
    }
  };

  return (
    <ResponsiveDialog
      open={true}
      onOpenChange={(open) => {
        if (!open) {
          onClose();
          setError(null);
          setTitle(livestream?.record?.title || "");
        }
      }}
      title="Update Stream Title"
      description="Update the title of the livestream."
      size="md"
      dismissible={false}
    >
      <View style={[{ padding: 16, paddingBottom: 0 }]}>
        <View style={[{ marginBottom: 16 }]}>
          <Text
            style={[
              { color: atoms.colors.gray[300], fontSize: 13, marginBottom: 8 },
            ]}
          >
            Stream Title
          </Text>
          <Textarea
            value={title}
            onChangeText={(text) => {
              setTitle(text);
              setError(null);
            }}
            placeholder="Enter stream title..."
            maxLength={140}
            multiline
            style={[
              {
                padding: 12,
                borderRadius: 8,
                backgroundColor: atoms.colors.neutral[800],
                color: atoms.colors.white,
                borderWidth: 1,
                borderColor: atoms.colors.neutral[600],
                minHeight: 100,
                fontSize: 16,
              },
            ]}
          />
          <Text
            style={[
              { color: atoms.colors.gray[400], fontSize: 12, marginTop: 4 },
            ]}
          >
            {title.length}/140 characters
          </Text>
        </View>

        {error && (
          <View
            style={[
              {
                backgroundColor: atoms.colors.red[900],
                padding: 12,
                borderRadius: 8,
                borderWidth: 1,
                borderColor: atoms.colors.red[700],
                marginBottom: 16,
              },
            ]}
          >
            <Text style={[{ color: atoms.colors.red[400], fontSize: 13 }]}>
              {error}
            </Text>
          </View>
        )}
      </View>

      <DialogFooter>
        <Button
          variant="secondary"
          onPress={() => {
            onClose();
            setError(null);
            setTitle(livestream?.record?.title || "");
          }}
          disabled={isLoading}
        >
          <Text>Cancel</Text>
        </Button>
        <Button
          variant="primary"
          onPress={handleUpdate}
          disabled={isLoading || !title.trim()}
        >
          <Text>{isLoading ? "Updating..." : "Update Title"}</Text>
        </Button>
      </DialogFooter>
    </ResponsiveDialog>
  );
}
