import Picker from "@emoji-mart/react";
import { AtSignIcon, ExternalLink, X } from "lucide-react-native";
import { useEffect, useMemo, useRef, useState } from "react";
import { Platform, Pressable, TextInput } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import {
  Button,
  Loader,
  Text,
  useChat,
  useCreateChatMessage,
  useLivestream,
  useReplyToMessage,
  useSetReplyToMessage,
  View,
} from "../../";
import {
  bg,
  flex,
  gap,
  h,
  layout,
  mb,
  mr,
  pl,
  pr,
  py,
  w,
} from "../../lib/theme/atoms";
import { usePDSAgent } from "../../streamplace-store/xrpc";
import { Textarea } from "../ui/textarea";
import { RenderChatMessage } from "./chat-message";
import { EmojiData, EmojiSuggestions } from "./emoji-suggestions";
import { MentionSuggestions } from "./mention-suggestions";

const COOL_EMOJI_LIST = [
  ..."😀🥸😍😘😁🥸😆🥸😜🥸😂😅🥸🙂🤫😱🥸🤣😗😄🥸😎🤓😲😯😰🥸😥🥸😣🥸😞😓🥸😩😩🥸😤🥱",
];

export function ChatBox({
  isPopout,
  chatBoxStyle,
  emojiData,
  setIsChatVisible,
}: {
  isPopout?: boolean;
  chatBoxStyle?: any;
  emojiData: EmojiData;
  setIsChatVisible?: (visible: boolean) => void;
}) {
  const [submitting, setSubmitting] = useState(false);
  const [message, setMessage] = useState("");
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

  let linfo = useLivestream();

  const chat = useChat();
  const createChatMessage = useCreateChatMessage();
  const replyTo = useReplyToMessage();
  const setReplyToMessage = useSetReplyToMessage();
  const textAreaRef = useRef<TextInput>(null);

  // are we logged in?

  let agent = usePDSAgent();

  if (!agent?.did) {
    <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
      <Text>Log in to chat.</Text>
    </View>;
  }

  const authors = useMemo(() => {
    if (!chat) return null;
    return chat.reduce((acc, msg) => {
      acc.set(msg.author.handle, msg.chatProfile);
      return acc;
    }, new Map<string, ChatMessageViewHydrated["chatProfile"]>());
  }, [chat]);

  const handleMentionSelect = (handle: string) => {
    const beforeAt = message.slice(0, message.lastIndexOf("@"));
    setMessage(`${beforeAt}@${handle} `);
    setShowSuggestions(false);
  };

  const handleEmojiSelect = (emoji: any) => {
    const beforeColon = message.slice(0, message.lastIndexOf(":"));
    setMessage(`${beforeColon}${emoji.skins[0]?.native} `);
    setShowEmojiSuggestions(false);
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
      if (searchText.length > 0) {
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
            if (emoji.name.toLowerCase().includes(searchText)) {
              return {
                emoji,
                sort: [6, emoji.name.toLowerCase().indexOf(searchText), 0],
              };
            }
            if (
              emoji.keywords &&
              emoji.keywords.some((keyword: string) =>
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

  const submit = () => {
    if (!message.trim()) return;
    setMessage("");
    setReplyToMessage(null);

    setSubmitting(true);
    createChatMessage({
      text: message,
      reply: replyTo || undefined,
    });
    setSubmitting(false);

    // if we press "send" button, we want the same action as pressing "Enter"
    // if we're already focused no need to do extra work
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
      {replyTo && (
        <View
          style={[
            layout.flex.row,
            layout.flex.alignCenter,
            layout.flex.spaceBetween,
            pl[2],
            pr[6],
            mr[6],
            mb[2],
            py[1],
            bg.gray[800],
            { borderRadius: 16 },
          ]}
        >
          <RenderChatMessage
            item={replyTo}
            showReply={false}
            userCache={authors || new Map()}
          />
          <Pressable onPress={() => setReplyToMessage(null)}>
            <View
              style={[
                layout.flex.row,
                layout.flex.alignCenter,
                layout.flex.justifyCenter,
                h[12],
                w[12],
                bg.gray[600],
                { borderRadius: 999 },
              ]}
            >
              <X size={24} />
            </View>
          </Pressable>
        </View>
      )}
      {showEmojiSelector && (
        <>
          {/* Overlay to catch outside clicks */}
          <Pressable
            style={{
              position: "absolute",
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              zIndex: 200,
            }}
            onPress={() => setShowEmojiSelector(false)}
          />
          <View
            style={{
              position: "absolute",
              bottom: "100%",
              left: 0,
              zIndex: 2001,
            }}
          >
            <Picker
              data={emojiData}
              onEmojiSelect={(e) => setMessage(message + e.native)}
            />
          </View>
        </>
      )}
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
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
              }
            }
          }}
          style={[chatBoxStyle]}
          // "submit" won't blur on enter
          submitBehavior="submit"
          placeholder="Type a message..."
        />
        <Button
          disabled={submitting}
          variant="secondary"
          style={{ borderRadius: 16, height: 36, minWidth: 80 }}
          onPress={submit}
        >
          {submitting ? <Loader /> : "Send"}
        </Button>
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
        />
      )}
      {Platform.OS === "web" && (
        <View
          style={[
            layout.flex.row,
            mb[2],
            gap.all[2],
            { justifyContent: "flex-end" },
          ]}
        >
          <Button
            variant="secondary"
            style={{ borderRadius: 16, height: 36, maxWidth: 36 }}
            onPress={() => {
              // if the last character is not @, add it
              !message.endsWith("@") && setMessage(message + "@");
              // get all the text after the last @
              const atIndex = message.lastIndexOf("@");
              const searchText = message.slice(atIndex + 1).toLowerCase();
              updateSuggestions(searchText);
              setShowSuggestions(true);
              // focus the textarea
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
              style={{ borderRadius: 16, height: 36, maxWidth: 36 }}
              onPress={() => setShowEmojiSelector(!showEmojiSelector)}
            >
              <Text>{COOL_EMOJI_LIST[emojiIconIndex]}</Text>
            </Button>
          </Pressable>
          {!isPopout && (
            <Button
              variant="secondary"
              style={{ borderRadius: 16, height: 36, maxWidth: 36 }}
              onPress={() => {
                if (!linfo) return;
                const u = new URL(window.location.href);
                u.pathname = `/chat-popout/${linfo?.author?.did}`;
                window.open(u.toString(), "_blank", "popup=true");
                setIsChatVisible?.(false);
              }}
            >
              <ExternalLink size={16} />
            </Button>
          )}
        </View>
      )}
    </View>
  );
}
