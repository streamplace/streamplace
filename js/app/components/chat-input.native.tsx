import {
  ChatFacet,
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
import { Pressable, Text, TextInput, View } from "react-native";

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

function buildEmoteMap(
  emojiPacks: RenderInputProps["emojiPacks"],
): Map<string, { aturi: string; cid: string }> {
  const map = new Map<string, { aturi: string; cid: string }>();
  if (!emojiPacks) return map;
  for (let i = emojiPacks.length - 1; i >= 0; i--) {
    for (const emote of emojiPacks[i].emoji) {
      map.set(emote.name, { aturi: emote.aturi, cid: emote.cid });
    }
  }
  return map;
}

function buildNamespacedMap(
  emojiPacks: RenderInputProps["emojiPacks"],
): Map<string, { aturi: string; cid: string }> {
  const map = new Map<string, { aturi: string; cid: string }>();
  if (!emojiPacks) return map;
  for (const pack of emojiPacks) {
    if (!pack.ownerHandle) continue;
    for (const emote of pack.emoji) {
      const key = `${pack.ownerHandle}/${emote.name}`;
      map.set(key, { aturi: emote.aturi, cid: emote.cid });
    }
  }
  return map;
}

function extractFacets(
  text: string,
  authors: RenderInputProps["authors"],
  emoteMap: Map<string, { aturi: string; cid: string }>,
  namespacedMap: Map<string, { aturi: string; cid: string }>,
): ChatFacet[] {
  const enc = new TextEncoder();
  const facets: ChatFacet[] = [];
  const authorMap = new Map(authors.map((a) => [a.handle.toLowerCase(), a]));
  const tokenRe = /(@\S+|:[a-zA-Z0-9._/-]+:)/g;
  let match: RegExpExecArray | null;
  while ((match = tokenRe.exec(text)) !== null) {
    const token = match[0];
    const byteStart = enc.encode(text.slice(0, match.index)).byteLength;
    const byteEnd = byteStart + enc.encode(token).byteLength;
    if (token.startsWith("@")) {
      const handle = token.slice(1).toLowerCase();
      const author = authorMap.get(handle);
      if (author?.did) {
        facets.push({
          index: { byteStart, byteEnd },
          features: [
            { $type: "app.bsky.richtext.facet#mention", did: author.did },
          ],
        } as unknown as ChatFacet);
      }
    } else {
      const inner = token.slice(1, -1);
      const slashIdx = inner.indexOf("/");
      let emoteData: { aturi: string; cid: string } | undefined;
      let name = inner;
      if (slashIdx !== -1) {
        emoteData = namespacedMap.get(inner.toLowerCase());
        name = inner.slice(slashIdx + 1);
      } else {
        emoteData = emoteMap.get(inner);
      }
      if (emoteData) {
        facets.push({
          index: { byteStart, byteEnd },
          features: [
            {
              $type: "place.stream.richtext.facet#emote",
              name,
              ref: { uri: emoteData.aturi, cid: emoteData.cid },
            },
          ],
        } as unknown as ChatFacet);
      }
    }
  }
  return facets;
}

function ChatNativeInput(props: RenderInputProps) {
  const [text, setText] = useState("");
  const [height, setHeight] = useState(43);
  const [mentionQuery, setMentionQuery] = useState<string | null>(null);
  const [emojiQuery, setEmojiQuery] = useState<string | null>(null);
  const [highlightedIndex, setHighlightedIndex] = useState(0);
  const [isEmojiPickerOpen, setIsEmojiPickerOpen] = useState(false);
  const highlightedIndexRef = useRef(0);
  const textRef = useRef("");

  const emoteMap = buildEmoteMap(props.emojiPacks);
  const namespacedMap = buildNamespacedMap(props.emojiPacks);

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

  const filteredCustomEmojis: any[] =
    emojiQuery !== null && props.emojiPacks
      ? props.emojiPacks.flatMap((pack) =>
          pack.emoji
            .filter((e) => e.name.toLowerCase().includes(emojiQuery))
            .map((e) => ({ type: "custom", ...e })),
        )
      : [];

  const filteredEmojis: any[] =
    emojiQuery !== null
      ? [
          ...filteredCustomEmojis,
          ...(props.emojiData ? searchEmojis(emojiQuery, props.emojiData) : []),
        ].slice(0, 10)
      : [];

  const filteredAuthorsRef = useRef(filteredAuthors);
  filteredAuthorsRef.current = filteredAuthors;
  const filteredEmojisRef = useRef(filteredEmojis);
  filteredEmojisRef.current = filteredEmojis;

  const hasAnySuggestions =
    filteredAuthors.size > 0 || filteredEmojis.length > 0;

  const clearSuggestions = useCallback(() => {
    setMentionQuery(null);
    setEmojiQuery(null);
    setHighlightedIndex(0);
    highlightedIndexRef.current = 0;
  }, []);

  const handleTextChange = useCallback(
    (val: string) => {
      setText(val);
      textRef.current = val;

      const atIdx = val.lastIndexOf("@");
      const colonIdx = val.lastIndexOf(":");

      if (atIdx !== -1 && (colonIdx === -1 || atIdx > colonIdx)) {
        const query = val.slice(atIdx + 1).toLowerCase();
        if (!query.includes(" ")) {
          setMentionQuery(query);
          setEmojiQuery(null);
          setHighlightedIndex(0);
          highlightedIndexRef.current = 0;
          return;
        }
      }

      if (colonIdx !== -1) {
        const query = val.slice(colonIdx + 1).toLowerCase();
        if (query.length >= 2 && !query.includes(" ") && !query.includes(":")) {
          setEmojiQuery(query);
          setMentionQuery(null);
          setHighlightedIndex(0);
          highlightedIndexRef.current = 0;
          return;
        }
      }

      clearSuggestions();
    },
    [clearSuggestions],
  );

  const handleMentionSelect = useCallback(
    (handle: string) => {
      const current = textRef.current;
      const atIdx = current.lastIndexOf("@");
      const newText =
        (atIdx !== -1 ? current.slice(0, atIdx) : current) + `@${handle} `;
      setText(newText);
      textRef.current = newText;
      clearSuggestions();
    },
    [clearSuggestions],
  );

  const handleEmojiSelect = useCallback(
    (emoji: any) => {
      const current = textRef.current;
      const colonIdx = current.lastIndexOf(":");
      const base = colonIdx !== -1 ? current.slice(0, colonIdx) : current;
      const insertion =
        emoji.type === "custom"
          ? `:${emoji.name}: `
          : getSkinNative(emoji, props.skinTone) + " ";
      const newText = base + insertion;
      setText(newText);
      textRef.current = newText;
      clearSuggestions();
    },
    [props.skinTone, clearSuggestions],
  );

  const handlePickerSelect = useCallback((emoji: SelectedEmoji) => {
    const current = textRef.current;
    const insertion =
      emoji.type === "standard" ? emoji.native + " " : `:${emoji.name}: `;
    const newText = current + insertion;
    setText(newText);
    textRef.current = newText;
    setIsEmojiPickerOpen(false);
  }, []);

  const handleSubmit = useCallback(() => {
    const current = textRef.current.trim();
    if (!current) return;
    const facets = extractFacets(
      current,
      props.authors,
      emoteMap,
      namespacedMap,
    );
    props.onSubmit({
      text: current,
      ...(facets.length > 0 ? { facets } : {}),
    });
    setText("");
    textRef.current = "";
    clearSuggestions();
    setTimeout(() => {
      setText("");
      textRef.current = "";
    }, 100);
  }, [props.authors, props.onSubmit, emoteMap, clearSuggestions]);

  const handleKeyPress = useCallback(
    (e: { nativeEvent: { key: string } }) => {
      if (e.nativeEvent.key === "Enter") {
        if (filteredAuthorsRef.current.size > 0) {
          const handles = Array.from(filteredAuthorsRef.current.keys());
          const handle = handles[highlightedIndexRef.current] ?? handles[0];
          if (handle) handleMentionSelect(handle);
          return;
        }
        if (filteredEmojisRef.current.length > 0) {
          const emoji =
            filteredEmojisRef.current[highlightedIndexRef.current] ??
            filteredEmojisRef.current[0];
          if (emoji) handleEmojiSelect(emoji);
          return;
        }
        handleSubmit();
      }
    },
    [handleMentionSelect, handleEmojiSelect, handleSubmit],
  );

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
        <TextInput
          style={{
            flex: 1,
            minHeight: 43,
            backgroundColor: "#111827",
            borderRadius: 12,
            borderWidth: 1,
            borderColor: "#374151",
            paddingHorizontal: 14,
            paddingVertical: 10,
            color: "white",
            fontSize: 14,
            lineHeight: 21,
          }}
          value={text}
          onChangeText={handleTextChange}
          onKeyPress={handleKeyPress}
          onContentSizeChange={(e) =>
            setHeight(Math.max(43, e.nativeEvent.contentSize.height))
          }
          multiline
          submitBehavior="submit"
          onSubmitEditing={handleSubmit}
          placeholder="Type a message..."
          placeholderTextColor="#6b7280"
        />
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
            accessibilityLabel="Open emote picker"
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
