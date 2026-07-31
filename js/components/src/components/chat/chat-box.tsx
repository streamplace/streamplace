import Graphemer from "graphemer";
import { AtSignIcon, ExternalLink, X } from "lucide-react-native";
import { env } from "process";
import { ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { Platform, Pressable, TextInput } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import { Button, Loader, Text, toast, useTheme, View } from "../../";
import { handleSlashCommand } from "../../lib/slash-commands";
import {
  createTeleport,
  registerTeleportCommand,
} from "../../lib/slash-commands/teleport";
import { StreamNotifications } from "../../lib/stream-notifications";
import { SystemMessages } from "../../lib/system-messages";
import {
  borders,
  flex,
  gap,
  h,
  layout,
  mb,
  pl,
  pr,
  py,
  r,
  w,
} from "../../lib/theme/atoms";
import {
  useAddSystemMessage,
  useChat,
  useChatDraft,
  useCreateChatMessage,
  useLivestream,
  useLivestreamStore,
  useProfile,
  useReplyToMessage,
  useSetChatDraft,
  useSetReplyToMessage,
} from "../../livestream-store";
import { useDID, usePDSAgent } from "../../streamplace-store";
import { Textarea } from "../ui/textarea";
import { RenderChatMessage } from "./chat-message";
import {
  EmojiData,
  EmojiSuggestions,
  getSkinNative,
} from "./emoji-suggestions";
import { MentionSuggestions } from "./mention-suggestions";
import { TeleportModal } from "./teleport-modal";

const COOL_EMOJI_LIST = [
  // @ts-ignore we can iterate through this just fine it seems
  ..."😀🥸😍😘😁🥸😆🥸😜🥸😂😅🥸🙂🤫😱🥸🤣😗😄🥸😎🤓😲😯😰🥸😥🥸😣🥸😞😓🥸😩😩🥸😤🥱",
];

const graphemer = new Graphemer();

export function ChatBox({
  isPopout,
  chatBoxStyle,
  emojiData,
  setIsChatVisible,
  onEmojiPickerToggle,
  emojiPicker,
  skinTone = 0,
  hideLogin = false,
  leftSlot,
}: {
  isPopout?: boolean;
  chatBoxStyle?: any;
  emojiData: EmojiData | null;
  setIsChatVisible?: (visible: boolean) => void;
  onEmojiPickerToggle?: () => void;
  emojiPicker?: (
    isOpen: boolean,
    onClose: () => void,
    onSelect: (emoji: any) => void,
  ) => ReactNode;
  skinTone?: number;
  hideLogin?: boolean;
  leftSlot?: ReactNode;
}) {
  const [submitting, setSubmitting] = useState(false);
  const [inputFocused, setInputFocused] = useState(false);
  const message = useChatDraft();
  const setMessage = useSetChatDraft();
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [showEmojiSuggestions, setShowEmojiSuggestions] = useState(false);
  const [showEmojiSelector, setShowEmojiSelector] = useState(false);
  const [emojiIconIndex, setEmojiIconIndex] = useState(
    Math.floor(Math.random() * COOL_EMOJI_LIST.length),
  );
  const [highlightedIndex, setHighlightedIndex] = useState(0);
  const [filteredAuthors, setFilteredAuthors] = useState<Map<string, any>>(
    new Map(),
  );
  const [filteredEmojis, setFilteredEmojis] = useState<any[]>([]);
  const [showTeleportModal, setShowTeleportModal] = useState(false);
  const isOverLimit = graphemer.countGraphemes(message) > 300;

  let linfo = useLivestream();
  const profile = useProfile();

  const { theme, zero: zt } = useTheme();

  const chat = useChat();
  const createChatMessage = useCreateChatMessage();
  const addSystemMessage = useAddSystemMessage();
  const replyTo = useReplyToMessage();
  const setReplyToMessage = useSetReplyToMessage();
  const textAreaRef = useRef<TextInput>(null);

  const pdsAgent = usePDSAgent();
  const userDID = useDID();
  const setActiveTeleportUri = useLivestreamStore(
    (state) => state.setActiveTeleportUri,
  );

  useEffect(() => {
    if (pdsAgent && userDID) {
      registerTeleportCommand(
        pdsAgent,
        userDID,
        () => (linfo ? { uri: linfo.uri, cid: linfo.cid } : null),
        setActiveTeleportUri,
        () => setShowTeleportModal(true),
      );
    }
  }, [pdsAgent, userDID, linfo, setActiveTeleportUri]);

  const authors = useMemo(() => {
    if (!chat) return null;
    return chat.reduce((acc, msg) => {
      // our fake system user "did"
      if (msg.author.did === "did:sys:system") return acc;
      if (acc.has(msg.author.handle)) return acc;
      acc.set(msg.author.handle, msg.chatProfile);
      return acc;
    }, new Map<string, ChatMessageViewHydrated["chatProfile"]>());
  }, [chat]);

  useEffect(() => {
    if (pdsAgent && linfo?.author?.did && pdsAgent.did === linfo.author.did) {
      registerTeleportCommand(
        pdsAgent,
        pdsAgent.did,
        () => (linfo ? { uri: linfo.uri, cid: linfo.cid } : null),
        setActiveTeleportUri,
        () => setShowTeleportModal(true),
      );
    }
  }, [pdsAgent, linfo, setActiveTeleportUri]);

  const handleMentionSelect = (handle: string) => {
    const beforeAt = message.slice(0, message.lastIndexOf("@"));
    setMessage(`${beforeAt}@${handle} `);
    setShowSuggestions(false);
  };

  const handleEmojiSelect = (emoji: any) => {
    console.log("[ChatBox] handleEmojiSelect", emoji);
    if (emoji.s) {
      const beforeColon = message.slice(0, message.lastIndexOf(":"));
      setMessage(`${beforeColon}${getSkinNative(emoji, skinTone)} `);
    } else if (emoji.type === "standard") {
      setMessage(message + emoji.native);
    } else if (emoji.type === "custom") {
      setMessage(message + `:${emoji.name}: `);
    }
    setShowEmojiSuggestions(false);
  };

  const handleTeleportSubmit = async (
    targetHandle: string,
    countdownSeconds: number,
  ) => {
    if (!pdsAgent || !userDID) return;

    const result = await createTeleport(
      pdsAgent,
      userDID,
      targetHandle,
      countdownSeconds,
      linfo ? { uri: linfo.uri, cid: linfo.cid } : undefined,
      setActiveTeleportUri,
    );

    if (!result.success && result.error) {
      SystemMessages.commandError(result.error);
    }
  };

  const updateSuggestions = (text: string) => {
    // Handle mentions
    const atIndex = text.lastIndexOf("@");
    if (atIndex !== -1 && authors) {
      const searchText = text.slice(atIndex + 1).toLowerCase();
      const filteredAuthorsMap = new Map(
        Array.from(authors.entries()).filter(([handle]) =>
          handle.toLowerCase().includes(searchText),
        ),
      );
      setFilteredAuthors(filteredAuthorsMap);
      setHighlightedIndex(0);
      setShowSuggestions(filteredAuthorsMap.size > 0);
      setShowEmojiSuggestions(false);
    } else {
      setShowSuggestions(false);
    }

    const colonIndex = text.lastIndexOf(":");
    if (colonIndex !== -1) {
      const searchText = text.slice(colonIndex + 1).toLowerCase();
      if (searchText.length >= 3 && !searchText.includes(" ")) {
        if (!emojiData) return;
        const aliasMatches = Object.entries(emojiData.aliases)
          .map(([alias, emojiId]) => {
            const aliasLower = alias.toLowerCase();
            if (aliasLower === searchText) {
              return { emojiId, alias, matchType: 0, index: 0 };
            } else if (aliasLower.startsWith(searchText)) {
              return { emojiId, alias, matchType: 1, index: 0 };
            } else if (aliasLower.includes(searchText)) {
              return {
                emojiId,
                alias,
                matchType: 2,
                index: aliasLower.indexOf(searchText),
              }; // includes
            }
            return null;
          })
          .filter(Boolean);

        // Map emojiId to best alias match info
        const bestAliasMatch: Record<
          string,
          { matchType: number; index: number; alias: string }
        > = {};
        for (const match of aliasMatches) {
          if (!match) continue;
          const prev = bestAliasMatch[match.emojiId];
          if (
            !prev ||
            match?.matchType < prev.matchType ||
            (match.matchType === prev.matchType && match.index < prev.index)
          ) {
            bestAliasMatch[match.emojiId] = match;
          }
        }

        // Collect all matching emojis by id, name, keywords, or alias
        const allEmojis = Object.values(emojiData.emojis);
        const filtered = allEmojis
          .map((emoji: any) => {
            // Check alias match
            const aliasMatch = bestAliasMatch[emoji.id];
            if (aliasMatch) {
              return {
                emoji,
                sort: [aliasMatch.matchType, aliasMatch.index, 0],
              };
            }
            // Check id, name, keywords
            if (emoji.id.toLowerCase() === searchText) {
              return { emoji, sort: [3, 0, 0] }; // exact id
            }
            if (emoji.id.toLowerCase().startsWith(searchText)) {
              return { emoji, sort: [4, 0, 0] }; // startsWith id
            }
            if (emoji.id.toLowerCase().includes(searchText)) {
              return {
                emoji,
                sort: [5, emoji.id.toLowerCase().indexOf(searchText), 0],
              }; // includes id
            }
            if (emoji.m.toLowerCase().includes(searchText)) {
              return {
                emoji,
                sort: [6, emoji.m.toLowerCase().indexOf(searchText), 0],
              };
            }
            if (
              emoji.k &&
              emoji.k.some((keyword: string) =>
                keyword.toLowerCase().includes(searchText),
              )
            ) {
              return { emoji, sort: [7, 0, 0] };
            }
            return null;
          })
          .filter(Boolean)
          // Remove duplicates by emoji id (keep best match)
          .reduce((acc: any[], curr: any) => {
            if (!acc.find((e) => e.emoji.id === curr.emoji.id)) {
              acc.push(curr);
            }
            return acc;
          }, [])
          // Sort by alias match type, then position, then fallback
          .sort((a, b) => {
            for (let i = 0; i < a.sort.length; ++i) {
              if (a.sort[i] !== b.sort[i]) return a.sort[i] - b.sort[i];
            }
            return 0;
          })
          .slice(0, 10) // Limit to 10 results
          .map((entry) => entry.emoji);

        setFilteredEmojis(filtered);
        setHighlightedIndex(0);
        setShowEmojiSuggestions(filtered.length > 0);
        setShowSuggestions(false);
      } else {
        setShowEmojiSuggestions(false);
      }
    } else {
      setShowEmojiSuggestions(false);
    }

    // If neither mention nor emoji, hide all suggestions
    if (atIndex === -1 && colonIndex === -1) {
      setShowSuggestions(false);
      setShowEmojiSuggestions(false);
    }
  };

  const submit = async () => {
    if (!message.trim()) return;
    if (graphemer.countGraphemes(message) > 300) {
      toast.show(
        "Message too long",
        "Please limit your message to 300 characters.",
        {
          variant: "error",
          duration: 3,
        },
      );
      return;
    }

    const messageText = message;
    setMessage("");
    setReplyToMessage(null);

    if (messageText.startsWith("/")) {
      const result = await handleSlashCommand(messageText);
      if (result.handled) {
        if (result.error) {
          console.error("Slash command error:", result.error);
          addSystemMessage(SystemMessages.commandError(result.error));
        }
        return;
      }
    }
    setSubmitting(true);

    try {
      const result = await handleSlashCommand(messageText);

      if (result.handled) {
        if (result.error) {
          console.error("Slash command error:", result.error);
        }
      } else {
        await createChatMessage({
          text: messageText,
          reply: replyTo || undefined,
        });
      }
    } catch (err) {
      console.error("Error submitting message:", err);
      toast.show("Failed to send message", "Please try again.", {
        variant: "error",
        duration: 3,
      });
    } finally {
      setSubmitting(false);
    }

    if (textAreaRef.current && !textAreaRef.current.isFocused()) {
      textAreaRef.current.focus();
      requestAnimationFrame(() => {
        textAreaRef.current?.focus();
      });
    }
  };
  useEffect(() => {
    if (replyTo && textAreaRef.current) {
      textAreaRef.current.focus();
    }
  }, [replyTo]);

  return (
    <View style={[layout.flex.column, flex.shrink[1], gap.all[2]]}>
      <TeleportModal
        open={showTeleportModal}
        onOpenChange={setShowTeleportModal}
        onSubmit={handleTeleportSubmit}
      />
      {replyTo && (
        <View
          style={[
            layout.flex.row,
            layout.flex.alignCenter,
            layout.flex.spaceBetween,
            pl[2],
            pr[1],
            mb[2],
            py[1],
            r["2xl"],
            zt.bg.card,
          ]}
        >
          <View style={{ flex: 1, minWidth: 0, marginRight: 8 }}>
            <RenderChatMessage
              item={replyTo}
              showReply={false}
              userCache={authors || new Map()}
            />
          </View>
          <Pressable
            onPress={() => setReplyToMessage(null)}
            style={[
              layout.flex.row,
              layout.flex.alignCenter,
              layout.flex.justifyCenter,
              h[8],
              w[8],
              zt.bg.muted,
              zt.border.border,
              borders.width.thin,
              { borderRadius: 999 },
            ]}
          >
            <X size={24} style={[zt.text.primaryForeground]} />
          </Pressable>
        </View>
      )}
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
        <View
          style={
            leftSlot
              ? [
                  layout.flex.row,
                  layout.flex.alignCenter,
                  { flex: 1 },
                  borders.width.thin,
                  { borderColor: theme.colors.borderSubtle },
                  { backgroundColor: theme.colors.input },
                  chatBoxStyle,
                  isOverLimit
                    ? {
                        borderColor: theme.colors.danger,
                        borderWidth: 2,
                        outline: "none",
                      }
                    : inputFocused
                      ? {
                          // Match every other field: focus lights the indigo
                          // ring, not a loud white border.
                          borderColor: theme.colors.ring,
                          outline: "none",
                        }
                      : null,
                  pr[2],
                ]
              : [{ flex: 1 }]
          }
        >
          {leftSlot}
          <Textarea
            ref={textAreaRef}
            numberOfLines={1}
            value={message}
            enterKeyHint="send"
            onSubmitEditing={(e) => {
              e.preventDefault();
              submit();
            }}
            multiline={false}
            onChangeText={(text) => {
              setMessage(text);
              updateSuggestions(text);
            }}
            onKeyPress={(k) => {
              if (k.nativeEvent.key === "Enter") {
                if (showSuggestions) {
                  k.preventDefault();
                  const handles = Array.from(filteredAuthors.keys());
                  if (handles.length > 0) {
                    handleMentionSelect(handles[highlightedIndex]);
                  }
                } else if (showEmojiSuggestions) {
                  k.preventDefault();
                  if (filteredEmojis.length > 0) {
                    handleEmojiSelect(filteredEmojis[highlightedIndex]);
                  }
                } else {
                  k.preventDefault();
                  submit();
                }
              } else if (k.nativeEvent.key === "Tab") {
                if (showSuggestions) {
                  k.preventDefault();
                  const handles = Array.from(filteredAuthors.keys());
                  if (handles.length > 0) {
                    handleMentionSelect(handles[highlightedIndex]);
                  }
                } else if (showEmojiSuggestions) {
                  k.preventDefault();
                  if (filteredEmojis.length > 0) {
                    handleEmojiSelect(filteredEmojis[highlightedIndex]);
                  }
                }
              } else if (k.nativeEvent.key === "ArrowUp") {
                if (showSuggestions || showEmojiSuggestions) {
                  k.preventDefault();
                  setHighlightedIndex((prev) => Math.max(prev - 1, 0));
                }
              } else if (k.nativeEvent.key === "ArrowDown") {
                if (showSuggestions) {
                  k.preventDefault();
                  setHighlightedIndex((prev) =>
                    Math.min(
                      prev + 1,
                      Array.from(filteredAuthors.keys()).length - 1,
                    ),
                  );
                } else if (showEmojiSuggestions) {
                  k.preventDefault();
                  setHighlightedIndex((prev) =>
                    Math.min(prev + 1, filteredEmojis.length - 1),
                  );
                }
              } else if (k.nativeEvent.key === "Escape") {
                if (showSuggestions || showEmojiSuggestions) {
                  k.preventDefault();
                  setShowSuggestions(false);
                  setShowEmojiSuggestions(false);
                } else if (replyTo) {
                  k.preventDefault();
                  setReplyToMessage(null);
                }
              }
            }}
            onFocus={() => setInputFocused(true)}
            onBlur={() => setInputFocused(false)}
            style={
              leftSlot
                ? [
                    {
                      flex: 1,
                      borderWidth: 0,
                      backgroundColor: "transparent",
                      outline: "none",
                    },
                  ]
                : [
                    chatBoxStyle,
                    isOverLimit && {
                      borderColor: theme.colors.danger,
                      borderWidth: 2,
                      outline: "none",
                    },
                  ]
            }
            // "submit" won't blur on enter
            submitBehavior="submit"
            placeholder="Type a message..."
          />
        </View>
        <View>
          <Button
            disabled={submitting}
            variant="secondary"
            width="min"
            style={{ borderRadius: theme.borderRadius.md, height: 43 }}
            onPress={submit}
          >
            {submitting ? <Loader /> : "Send"}
          </Button>
        </View>
      </View>
      {showSuggestions && (
        <MentionSuggestions
          authors={filteredAuthors || new Map()}
          highlightedIndex={highlightedIndex}
          onSelect={handleMentionSelect}
        />
      )}
      {showEmojiSuggestions && (
        <EmojiSuggestions
          emojis={filteredEmojis}
          highlightedIndex={highlightedIndex}
          onSelect={handleEmojiSelect}
          skinTone={skinTone}
        />
      )}
      {Platform.OS === "web" && (
        <View
          style={[
            layout.flex.row,
            mb[2],
            gap.all[2],
            { justifyContent: "flex-end", position: "relative" },
          ]}
        >
          {emojiPicker?.(
            showEmojiSelector,
            () => setShowEmojiSelector(false),
            handleEmojiSelect,
          )}
          {env.NODE_ENV === "development" && (
            <Button
              variant="secondary"
              style={{ borderRadius: theme.borderRadius.md }}
              width="min"
              onPress={() => {
                StreamNotifications.teleport({
                  targetHandle: "test.bsky.social",
                  targetDID: "did:plc:test",
                  countdown: 30,
                  canCancel: true,
                  onDismiss: (reason) =>
                    console.log("teleport dismissed:", reason),
                });
              }}
            >
              Test Notification
            </Button>
          )}
          <Button
            variant="secondary"
            style={{
              borderRadius: theme.borderRadius.md,
              maxWidth: 44,
              aspectRatio: 1,
            }}
            aria-label="Insert Mention"
            onPress={() => {
              !message.endsWith("@") && setMessage(message + "@");
              const atIndex = message.lastIndexOf("@");
              const searchText = message.slice(atIndex + 1).toLowerCase();
              updateSuggestions(searchText);
              setShowSuggestions(true);
              textAreaRef.current?.focus();
            }}
          >
            <AtSignIcon size={20} color="white" />
          </Button>
          <Pressable
            onHoverOut={() => {
              setEmojiIconIndex(
                Math.floor(Math.random() * COOL_EMOJI_LIST.length),
              );
            }}
          >
            <Button
              variant="secondary"
              id="web-emoji-picker-btn"
              aria-label="Insert Emoji"
              style={{
                borderRadius: theme.borderRadius.md,
                maxWidth: 44,
                aspectRatio: 1,
              }}
              onPress={() => {
                onEmojiPickerToggle
                  ? onEmojiPickerToggle()
                  : setShowEmojiSelector(!showEmojiSelector);
              }}
            >
              <Text>{COOL_EMOJI_LIST[emojiIconIndex]}</Text>
            </Button>
          </Pressable>
          {!isPopout && (
            <Button
              // @ts-ignore this should work fine on web and get ignored on mobile
              href={`/chat-popout/${linfo?.author?.did}`}
              variant="secondary"
              aria-label="Popout Chat"
              style={{
                borderRadius: theme.borderRadius.md,
                maxWidth: 44,
                aspectRatio: 1,
              }}
              onPress={() => {
                const did = linfo?.author?.did ?? profile?.did;
                if (did) {
                  const u = new URL(window.location.href);
                  u.pathname = `/chat-popout/${did}`;
                  window.open(
                    u.toString(),
                    "_blank",
                    "popup=true,width=480,height=600",
                  );
                }
                setIsChatVisible?.(false);
              }}
            >
              <ExternalLink color={theme.colors.primaryForeground} size={16} />
            </Button>
          )}
        </View>
      )}
    </View>
  );
}
