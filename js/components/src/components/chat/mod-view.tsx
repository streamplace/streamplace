import { TriggerRef, useRootContext } from "@rn-primitives/dropdown-menu";
import { forwardRef, useEffect, useRef, useState } from "react";
import { gap, mr, w } from "../../lib/theme/atoms";
import { usePlayerStore } from "../../player-store";
import {
  useCreateBlockRecord,
  useCreateHideChatRecord,
} from "../../streamplace-store/block";
import { usePDSAgent } from "../../streamplace-store/xrpc";

import { Linking } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import { useStreamplaceStore } from "../../streamplace-store";
import {
  atoms,
  DropdownMenu,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
  layout,
  ResponsiveDropdownMenuContent,
  Text,
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
                    @{message.author.handle}: {message.record.text}
                  </Text>
                </View>
              </DropdownMenuItem>
            </DropdownMenuGroup>

            {/* TODO: Checking for non-owner moderators */}
            {channelId === handle && (
              <DropdownMenuGroup title={`Moderation actions`}>
                <DropdownMenuItem
                  disabled={isHideLoading || messageRemoved}
                  onPress={() => {
                    if (isHideLoading || messageRemoved) return;
                    createHideChat(message.uri)
                      .then((r) => setMessageRemoved(true))
                      .catch((e) => console.error(e));
                  }}
                >
                  <Text
                    color={
                      isHideLoading || messageRemoved ? "muted" : "destructive"
                    }
                  >
                    {isHideLoading
                      ? "Removing..."
                      : messageRemoved
                        ? "Message removed"
                        : "Remove this message"}
                  </Text>
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={message.author.did === agent?.did || isBlockLoading}
                  onPress={() => {
                    createBlock(message.author.did)
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
                        : `Block user @${message.author.handle} from this channel`}
                    </Text>
                  )}
                </DropdownMenuItem>
              </DropdownMenuGroup>
            )}

            <DropdownMenuGroup title={`User actions`}>
              <DropdownMenuItem
                onPress={() => {
                  Linking.openURL(
                    `https://${BSKY_FRONTEND_DOMAIN}/profile/${message.author.handle}`,
                  );
                }}
              >
                <Text color="primary">View user on {BSKY_FRONTEND_DOMAIN}</Text>
              </DropdownMenuItem>
              <ReportButton
                message={message}
                setReportModalOpen={setReportModalOpen}
                setReportSubject={setReportSubject}
              />
            </DropdownMenuGroup>
          </>
        )}
      </ResponsiveDropdownMenuContent>
    </DropdownMenu>
  );
});

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
