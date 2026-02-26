import {
  EmojiData,
  EmojiSuggestions,
  getSkinNative,
  MentionSuggestions,
  RenderInputProps,
} from "@streamplace/components";
import {
  EmojiPicker,
  SelectedEmoji,
} from "components/emoji-picker/emoji-picker";
import { useCallback, useRef, useState } from "react";
import { Pressable, Text, View } from "react-native";
import ChatEditor, { RichTextResult } from "./chat-editor.dom";

function searchEmojis(query: string, emojiData: EmojiData) {
  const aliasMatches: Record<string, { matchType: number; index: number }> = {};
  for (const [alias, emojiId] of Object.entries(emojiData.aliases)) {
    const a = alias.toLowerCase();
    if (a === query) aliasMatches[emojiId] = { matchType: 0, index: 0 };
    else if (a.startsWith(query) && !aliasMatches[emojiId])
      aliasMatches[emojiId] = { matchType: 1, index: 0 };
    else if (a.includes(query) && !aliasMatches[emojiId])
      aliasMatches[emojiId] = { matchType: 2, index: a.indexOf(query) };
  }
  return Object.values(emojiData.emojis)
    .map((emoji) => {
      const am = aliasMatches[emoji.id];
      if (am) return { emoji, sort: [am.matchType, am.index] };
      if (emoji.id.toLowerCase() === query) return { emoji, sort: [3, 0] };
      if (emoji.id.toLowerCase().startsWith(query))
        return { emoji, sort: [4, 0] };
      if (emoji.id.toLowerCase().includes(query))
        return { emoji, sort: [5, emoji.id.toLowerCase().indexOf(query)] };
      if (emoji.m.toLowerCase().includes(query))
        return { emoji, sort: [6, emoji.m.toLowerCase().indexOf(query)] };
      return null;
    })
    .filter(
      (x): x is { emoji: (typeof emojiData.emojis)[string]; sort: number[] } =>
        x !== null,
    )
    .sort((a, b) => {
      for (let i = 0; i < a.sort.length; i++) {
        if (a.sort[i] !== b.sort[i]) return a.sort[i] - b.sort[i];
      }
      return 0;
    })
    .slice(0, 10)
    .map((x) => x.emoji);
}

function ChatNativeInput(props: RenderInputProps) {
  const [height, setHeight] = useState(43);
  const [mentionQuery, setMentionQuery] = useState<string | null>(null);
  const [emojiQuery, setEmojiQuery] = useState<string | null>(null);
  const [highlightedIndex, setHighlightedIndex] = useState(0);
  const [isEmojiPickerOpen, setIsEmojiPickerOpen] = useState(false);
  const highlightedIndexRef = useRef(0);
  const insertSeqRef = useRef(0);
  const [internalInsert, setInternalInsert] =
    useState<RenderInputProps["insertElement"]>(undefined);

  // Keep refs so onEnter closure always sees current suggestion state
  const filteredAuthorsRef = useRef<
    Map<string, (typeof props.authors)[number] | undefined>
  >(new Map());
  const filteredEmojisRef = useRef<
    typeof props.emojiData extends null
      ? never
      : ReturnType<typeof searchEmojis>
  >([]);

  const filteredAuthors: Map<
    string,
    (typeof props.authors)[number] | undefined
  > =
    mentionQuery !== null
      ? new Map(
          props.authors
            .filter((a) => a.handle.toLowerCase().includes(mentionQuery))
            .map((a) => [a.handle, a]),
        )
      : new Map();

  const filteredEmojis =
    emojiQuery !== null && props.emojiData
      ? searchEmojis(emojiQuery, props.emojiData)
      : [];

  filteredAuthorsRef.current = filteredAuthors;
  filteredEmojisRef.current = filteredEmojis as any;

  const hasAnySuggestions =
    filteredAuthors.size > 0 || filteredEmojis.length > 0;

  const clearSuggestions = useCallback(() => {
    setMentionQuery(null);
    setEmojiQuery(null);
    setHighlightedIndex(0);
    highlightedIndexRef.current = 0;
  }, []);

  const handleMentionSelect = useCallback(
    (handle: string) => {
      const author = filteredAuthorsRef.current.get(handle);
      const seq = ++insertSeqRef.current;
      setInternalInsert({
        type: "mention",
        handle,
        did: author?.did ?? null,
        seq,
      });
      clearSuggestions();
    },
    [clearSuggestions],
  );

  const handleEmojiSelect = useCallback(
    (emoji: ReturnType<typeof searchEmojis>[number]) => {
      const native = getSkinNative(emoji, props.skinTone);
      const seq = ++insertSeqRef.current;
      setInternalInsert({
        type: "emoji",
        emojiId: emoji.id,
        native,
        text: native,
        seq,
      });
      clearSuggestions();
    },
    [props.skinTone, clearSuggestions],
  );

  const handlePickerSelect = useCallback((emoji: SelectedEmoji) => {
    const seq = ++insertSeqRef.current;
    if (emoji.type === "standard") {
      setInternalInsert({
        type: "emoji",
        emojiId: emoji.native,
        native: emoji.native,
        text: emoji.native,
        seq,
      });
    } else {
      setInternalInsert({
        type: "emoji",
        emojiId: emoji.name,
        native: null,
        imageUrl: emoji.imageUrl,
        text: `:${emoji.name}:`,
        seq,
      });
    }
    setIsEmojiPickerOpen(false);
  }, []);

  // Called by the editor when Enter is pressed with no internal (webview) suggestions active.
  // If native suggestions are showing, confirm the highlighted one; otherwise submit.
  const handleEnter = useCallback(
    (msg: RichTextResult) => {
      const authors = filteredAuthorsRef.current;
      const emojis = filteredEmojisRef.current;
      if (authors.size > 0) {
        const handles = Array.from(authors.keys());
        const handle = handles[highlightedIndexRef.current] ?? handles[0];
        if (handle) handleMentionSelect(handle);
      } else if (emojis.length > 0) {
        const emoji = emojis[highlightedIndexRef.current] ?? emojis[0];
        if (emoji) handleEmojiSelect(emoji);
      } else if (msg.text) {
        // Submit — tell the editor to clear via insertElement
        const seq = ++insertSeqRef.current;
        setInternalInsert({ type: "clear", seq });
        props.onSubmit(msg as Parameters<typeof props.onSubmit>[0]);
      }
    },
    [handleMentionSelect, handleEmojiSelect, props.onSubmit],
  );

  // Prefer external insertElement (emoji picker) when seq changes, otherwise use internal
  const activeInsert = props.insertElement ?? internalInsert;

  const hasEmojiPacks = (props.emojiPacks?.length ?? 0) > 0;

  return (
    <View style={{ width: "100%", position: "relative" }}>
      {hasAnySuggestions && (
        <View
          style={{
            position: "absolute",
            bottom: height,
            left: 0,
            right: 0,
            zIndex: 999,
          }}
        >
          {filteredAuthors.size > 0 ? (
            <MentionSuggestions
              authors={filteredAuthors as any}
              highlightedIndex={highlightedIndex}
              onSelect={handleMentionSelect}
            />
          ) : (
            <EmojiSuggestions
              emojis={filteredEmojis}
              highlightedIndex={highlightedIndex}
              onSelect={handleEmojiSelect}
              skinTone={props.skinTone}
            />
          )}
        </View>
      )}
      <View style={{ flexDirection: "row", alignItems: "flex-end", gap: 6 }}>
        <View style={{ flex: 1, height }}>
          <ChatEditor
            authors={props.authors}
            emojiData={props.emojiData}
            skinTone={props.skinTone}
            onSubmit={handleEnter}
            insertElement={activeInsert}
            onDOMLayout={({ height: h }) => setHeight(h)}
            onMentionQuery={(q) => {
              setMentionQuery(q);
              setHighlightedIndex(0);
              highlightedIndexRef.current = 0;
            }}
            onEmojiQuery={(q) => {
              setEmojiQuery(q);
              setHighlightedIndex(0);
              highlightedIndexRef.current = 0;
            }}
            onEnter={handleEnter}
          />
        </View>
        {hasEmojiPacks && (
          <Pressable
            onPress={() => setIsEmojiPickerOpen((v) => !v)}
            style={({ pressed }) => ({
              width: height,
              height,
              borderRadius: 12,
              backgroundColor: isEmojiPickerOpen
                ? "rgba(99,102,241,0.3)"
                : pressed
                  ? "rgba(255,255,255,0.15)"
                  : "rgba(255,255,255,0.08)",
              alignItems: "center",
              justifyContent: "center",
            })}
            accessibilityLabel="Open emoji picker"
          >
            <Text style={{ fontSize: 20 }}>😀</Text>
          </Pressable>
        )}
      </View>
      {hasEmojiPacks && (
        <EmojiPicker
          isOpen={isEmojiPickerOpen}
          onClose={() => setIsEmojiPickerOpen(false)}
          onSelect={handlePickerSelect}
          emojiPacks={props.emojiPacks ?? []}
        />
      )}
    </View>
  );
}

export function renderChatInput(props: RenderInputProps) {
  return <ChatNativeInput {...props} />;
}
