import { X } from "lucide-react-native";
import { useEffect, useMemo, useRef, useState } from "react";
import { Pressable, TextInput } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import {
  Button,
  Loader,
  Text,
  useChat,
  useCreateChatMessage,
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
import { MentionSuggestions } from "./mention-suggestions";

export function ChatBox({
  isPopout,
  chatBoxStyle,
}: {
  isPopout?: boolean;
  chatBoxStyle?: any;
}) {
  const [submitting, setSubmitting] = useState(false);
  const [message, setMessage] = useState("");
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [highlightedIndex, setHighlightedIndex] = useState(0);
  const [filteredAuthors, setFilteredAuthors] = useState<Map<string, any>>(
    new Map(),
  );

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

  const updateSuggestions = (text: string) => {
    const atIndex = text.lastIndexOf("@");
    if (atIndex === -1 || !authors) {
      setShowSuggestions(false);
      return;
    }

    const searchText = text.slice(atIndex + 1).toLowerCase();

    const filteredAuthorsMap = new Map(
      Array.from(authors.entries()).filter(([handle]) =>
        handle.toLowerCase().includes(searchText),
      ),
    );

    setFilteredAuthors(filteredAuthorsMap);

    setHighlightedIndex(0);
    setShowSuggestions(filteredAuthorsMap.size > 0);
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
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
        <Textarea
          ref={textAreaRef}
          numberOfLines={1}
          value={message}
          enterKeyHint="send"
          onSubmitEditing={submit}
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
              } else submit();
            } else if (k.nativeEvent.key === "ArrowUp") {
              setHighlightedIndex((prev) => Math.max(prev - 1, 0));
            } else if (k.nativeEvent.key === "ArrowDown") {
              setHighlightedIndex((prev) =>
                Math.min(
                  prev + 1,
                  Array.from(filteredAuthors.keys()).length - 1,
                ),
              );
            }
          }}
          style={[chatBoxStyle]}
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
          authors={filteredAuthors || []}
          highlightedIndex={highlightedIndex}
          onSelect={handleMentionSelect}
        />
      )}
    </View>
  );
}
