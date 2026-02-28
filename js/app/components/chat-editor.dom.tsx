"use dom";

import {
  InitialConfigType,
  LexicalComposer,
} from "@lexical/react/LexicalComposer";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import { ContentEditable } from "@lexical/react/LexicalContentEditable";
import { LexicalErrorBoundary } from "@lexical/react/LexicalErrorBoundary";
import { HistoryPlugin } from "@lexical/react/LexicalHistoryPlugin";
import { PlainTextPlugin } from "@lexical/react/LexicalPlainTextPlugin";
import {
  $getRoot,
  $getSelection,
  $isRangeSelection,
  $isTextNode,
  COMMAND_PRIORITY_HIGH,
  KEY_ARROW_DOWN_COMMAND,
  KEY_ARROW_UP_COMMAND,
  KEY_ENTER_COMMAND,
  KEY_ESCAPE_COMMAND,
  KEY_TAB_COMMAND,
  LexicalEditor,
  LexicalNode,
  NodeKey,
  SerializedTextNode,
  TextNode,
} from "lexical";
import { useCallback, useEffect, useRef, useState } from "react";

// ── useSize ───────────────────────────────────────────────────────────────────

function useSize(
  callback: ((size: { width: number; height: number }) => void) | undefined,
) {
  useEffect(() => {
    if (!callback) return;
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        callback({ width, height });
      }
    });
    observer.observe(document.body);
    callback({
      width: document.body.clientWidth,
      height: document.body.clientHeight,
    });
    return () => observer.disconnect();
  }, [callback]);
}

// ── Types ────────────────────────────────────────────────────────────────────

export interface AuthorEntry {
  handle: string;
  did?: string;
  color?: { red: number; green: number; blue: number };
}

export interface EmojiSkin {
  n: string;
}

export interface EmojiEntry {
  id: string;
  m: string;
  k: string[];
  s: EmojiSkin[];
}

export interface EmojiData {
  emojis: { [key: string]: EmojiEntry };
  aliases: { [key: string]: string };
}

interface ChatEditorProps {
  authors: AuthorEntry[];
  emojiData: EmojiData | null;
  skinTone: number;
  placeholder?: string;
  onSubmit: (msg: RichTextResult) => void;
  insertElement?: InsertElement;
  onDOMLayout?: (size: { width: number; height: number }) => void;
  onMentionQuery?: (query: string | null) => void;
  onEmojiQuery?: (query: string | null) => void;
  onEnter?: (msg: RichTextResult) => void;
}

// ── MentionNode ──────────────────────────────────────────────────────────────

interface SerializedMentionNode extends SerializedTextNode {
  type: "mention";
  handle: string;
  did: string | null;
}

class MentionNode extends TextNode {
  __handle: string;
  __did: string | null;

  constructor(handle: string, did: string | null, key?: NodeKey) {
    super(`@${handle}`, key);
    this.__handle = handle;
    this.__did = did;
  }

  static getType(): string {
    return "mention";
  }

  static clone(node: MentionNode): MentionNode {
    return new MentionNode(node.__handle, node.__did, node.__key);
  }

  createDOM(): HTMLElement {
    const el = document.createElement("span");
    el.className = "mention-chip";
    el.textContent = `@${this.__handle}`;
    el.style.cssText = [
      "background: rgba(99,102,241,0.2)",
      "color: #a5b4fc",
      "border-radius: 4px",
      "padding: 0 4px",
      "font-weight: 600",
      "white-space: nowrap",
    ].join(";");
    return el;
  }

  updateDOM(prevNode: MentionNode, dom: HTMLElement): boolean {
    if (prevNode.__handle !== this.__handle) {
      dom.textContent = `@${this.__handle}`;
    }
    return false;
  }

  isToken(): boolean {
    return true;
  }

  exportJSON(): SerializedMentionNode {
    return {
      ...super.exportJSON(),
      type: "mention",
      handle: this.__handle,
      did: this.__did,
    };
  }

  static importJSON(json: SerializedMentionNode): MentionNode {
    return new MentionNode(json.handle, json.did ?? null);
  }
}

function $createMentionNode(handle: string, did: string | null): MentionNode {
  return new MentionNode(handle, did);
}

function $isMentionNode(
  node: LexicalNode | null | undefined,
): node is MentionNode {
  return node instanceof MentionNode;
}

// ── EmojiNode ────────────────────────────────────────────────────────────────

interface SerializedEmojiNode extends SerializedTextNode {
  type: "emoji";
  emojiId: string;
  native: string | null;
  aturi: string | null;
  cid: string | null;
  imageUrl: string | null;
}

class EmojiNode extends TextNode {
  __emojiId: string;
  __native: string | null;
  __aturi: string | null;
  __cid: string | null;
  __imageUrl: string | null;

  constructor(
    text: string,
    emojiId: string,
    native: string | null,
    aturi: string | null,
    cid: string | null,
    imageUrl: string | null = null,
    key?: NodeKey,
  ) {
    super(text, key);
    this.__emojiId = emojiId;
    this.__native = native;
    this.__aturi = aturi;
    this.__cid = cid;
    this.__imageUrl = imageUrl;
  }

  static getType(): string {
    return "emoji";
  }

  static clone(node: EmojiNode): EmojiNode {
    return new EmojiNode(
      node.__text,
      node.__emojiId,
      node.__native,
      node.__aturi,
      node.__cid,
      node.__imageUrl,
      node.__key,
    );
  }

  createDOM(): HTMLElement {
    const el = document.createElement("span");
    el.className = "emoji-node";
    el.style.cssText =
      "white-space: nowrap; display: inline-flex; align-items: center; vertical-align: middle";
    const src = this.__imageUrl;
    if (src) {
      const img = document.createElement("img");
      img.src = src;
      img.alt = this.__text;
      img.title = this.__text;
      img.style.cssText =
        "width: 1.4em; height: 1.4em; object-fit: contain; vertical-align: middle";
      el.appendChild(img);
    } else {
      el.textContent = this.__text;
    }
    return el;
  }

  updateDOM(prevNode: EmojiNode, dom: HTMLElement): boolean {
    const src = this.__imageUrl;
    const prevSrc = prevNode.__imageUrl;
    if (prevSrc !== src || prevNode.__text !== this.__text) {
      dom.innerHTML = "";
      if (src) {
        const img = document.createElement("img");
        img.src = src;
        img.alt = this.__text;
        img.title = this.__text;
        img.style.cssText =
          "width: 1.4em; height: 1.4em; object-fit: contain; vertical-align: middle";
        dom.appendChild(img);
      } else {
        dom.textContent = this.__text;
      }
    }
    return false;
  }

  isToken(): boolean {
    return true;
  }

  exportJSON(): SerializedEmojiNode {
    return {
      ...super.exportJSON(),
      type: "emoji",
      emojiId: this.__emojiId,
      native: this.__native,
      aturi: this.__aturi,
      cid: this.__cid,
      imageUrl: this.__imageUrl,
    };
  }

  static importJSON(json: SerializedEmojiNode): EmojiNode {
    return new EmojiNode(
      json.text,
      json.emojiId,
      json.native,
      json.aturi ?? null,
      json.cid ?? null,
      json.imageUrl ?? null,
    );
  }
}

function $createEmojiNode(
  text: string,
  emojiId: string,
  native: string | null,
  aturi: string | null = null,
  cid: string | null = null,
  imageUrl: string | null = null,
): EmojiNode {
  return new EmojiNode(text, emojiId, native, aturi, cid, imageUrl);
}

// ── Rich text extraction ─────────────────────────────────────────────────────

export interface RichTextFacet {
  index: { byteStart: number; byteEnd: number };
  features: (
    | { $type: "app.bsky.richtext.facet#mention"; did: string }
    | {
        $type: "place.stream.richtext.facet#emote";
        name: string;
        ref?: { uri: string; cid: string };
        imageUrl?: string;
      }
  )[];
}

export interface RichTextResult {
  text: string;
  facets: RichTextFacet[];
}

function extractRichText(editor: LexicalEditor): RichTextResult {
  return editor.getEditorState().read(() => {
    const enc = new TextEncoder();
    const root = $getRoot();
    const parts: string[] = [];
    const facets: RichTextFacet[] = [];

    for (const child of root.getChildren()) {
      for (const node of (child as any).getChildren()) {
        if ($isMentionNode(node)) {
          const nodeText = `@${node.__handle}`;
          const byteStart = enc.encode(parts.join("")).byteLength;
          const byteEnd = byteStart + enc.encode(nodeText).byteLength;
          parts.push(nodeText);
          if (node.__did) {
            facets.push({
              index: { byteStart, byteEnd },
              features: [
                { $type: "app.bsky.richtext.facet#mention", did: node.__did },
              ],
            });
          }
        } else if (node instanceof EmojiNode) {
          const nodeText = node.getTextContent();
          const byteStart = enc.encode(parts.join("")).byteLength;
          const byteEnd = byteStart + enc.encode(nodeText).byteLength;
          parts.push(nodeText);
          if (!node.__native) {
            facets.push({
              index: { byteStart, byteEnd },
              features: [
                {
                  $type: "place.stream.richtext.facet#emote",
                  name: node.__emojiId,
                  ...(node.__aturi && node.__cid
                    ? { ref: { uri: node.__aturi, cid: node.__cid } }
                    : {}),
                },
              ],
            });
          }
        } else if ($isTextNode(node)) {
          parts.push(node.getTextContent());
        }
      }
    }

    return { text: parts.join("").trim(), facets };
  });
}

// ── Emoji search helpers ─────────────────────────────────────────────────────

function searchEmojis(query: string, emojiData: EmojiData): EmojiEntry[] {
  if (!query || query.length < 3) return [];

  const aliasMatches: Record<
    string,
    { matchType: number; index: number; alias: string }
  > = {};
  for (const [alias, emojiId] of Object.entries(emojiData.aliases)) {
    const aliasLower = alias.toLowerCase();
    if (aliasLower === query) {
      aliasMatches[emojiId] = { matchType: 0, index: 0, alias };
    } else if (aliasLower.startsWith(query)) {
      if (!aliasMatches[emojiId] || aliasMatches[emojiId].matchType > 1) {
        aliasMatches[emojiId] = { matchType: 1, index: 0, alias };
      }
    } else if (aliasLower.includes(query)) {
      const idx = aliasLower.indexOf(query);
      if (!aliasMatches[emojiId] || aliasMatches[emojiId].matchType > 2) {
        aliasMatches[emojiId] = { matchType: 2, index: idx, alias };
      }
    }
  }

  const allEmojis = Object.values(emojiData.emojis);
  return allEmojis
    .map((emoji) => {
      const aliasMatch = aliasMatches[emoji.id];
      if (aliasMatch)
        return { emoji, sort: [aliasMatch.matchType, aliasMatch.index, 0] };
      if (emoji.id.toLowerCase() === query) return { emoji, sort: [3, 0, 0] };
      if (emoji.id.toLowerCase().startsWith(query))
        return { emoji, sort: [4, 0, 0] };
      if (emoji.id.toLowerCase().includes(query))
        return { emoji, sort: [5, emoji.id.toLowerCase().indexOf(query), 0] };
      if (emoji.m.toLowerCase().includes(query))
        return { emoji, sort: [6, emoji.m.toLowerCase().indexOf(query), 0] };
      if (emoji.k?.some((k) => k.toLowerCase().includes(query)))
        return { emoji, sort: [7, 0, 0] };
      return null;
    })
    .filter((x): x is { emoji: EmojiEntry; sort: number[] } => x !== null)
    .reduce<{ emoji: EmojiEntry; sort: number[] }[]>((acc, curr) => {
      if (!acc.find((e) => e.emoji.id === curr.emoji.id)) acc.push(curr);
      return acc;
    }, [])
    .sort((a, b) => {
      for (let i = 0; i < a.sort.length; i++) {
        if (a.sort[i] !== b.sort[i]) return a.sort[i] - b.sort[i];
      }
      return 0;
    })
    .slice(0, 10)
    .map((x) => x.emoji);
}

// ── Suggestion detection ─────────────────────────────────────────────────────

function getTextBeforeCursor(): string {
  const selection = $getSelection();
  if (!$isRangeSelection(selection) || !selection.isCollapsed()) return "";
  const anchor = selection.anchor;
  const node = anchor.getNode();
  if (!$isTextNode(node)) return "";
  return node.getTextContent().slice(0, anchor.offset);
}

// ── Plugins ──────────────────────────────────────────────────────────────────

interface SuggestionsState {
  mentionQuery: string | null;
  emojiQuery: string | null;
  highlightedIndex: number;
  filteredAuthors: AuthorEntry[];
  filteredEmojis: EmojiEntry[];
}

function useSuggestions(
  authors: AuthorEntry[],
  emojiData: EmojiData | null,
): [
  SuggestionsState,
  React.Dispatch<React.SetStateAction<SuggestionsState>>,
  (text: string) => void,
] {
  const [state, setState] = useState<SuggestionsState>({
    mentionQuery: null,
    emojiQuery: null,
    highlightedIndex: 0,
    filteredAuthors: [],
    filteredEmojis: [],
  });

  const update = useCallback(
    (textBefore: string) => {
      const atIdx = textBefore.lastIndexOf("@");
      const colonIdx = textBefore.lastIndexOf(":");

      if (atIdx !== -1 && (colonIdx === -1 || atIdx > colonIdx)) {
        const query = textBefore.slice(atIdx + 1).toLowerCase();
        // Stop if there's a space between @ and cursor — mention was completed or abandoned
        if (!query.includes(" ")) {
          const filtered = authors.filter((a) =>
            a.handle.toLowerCase().includes(query),
          );
          setState({
            mentionQuery: query,
            emojiQuery: null,
            highlightedIndex: 0,
            filteredAuthors: filtered,
            filteredEmojis: [],
          });
          return;
        }
      }

      if (colonIdx !== -1) {
        const query = textBefore.slice(colonIdx + 1).toLowerCase();
        // Stop if there's a space between : and cursor
        if (query.length >= 3 && !query.includes(" ") && emojiData) {
          const filtered = searchEmojis(query, emojiData);
          setState({
            mentionQuery: null,
            emojiQuery: query,
            highlightedIndex: 0,
            filteredAuthors: [],
            filteredEmojis: filtered,
          });
          return;
        }
      }

      setState({
        mentionQuery: null,
        emojiQuery: null,
        highlightedIndex: 0,
        filteredAuthors: [],
        filteredEmojis: [],
      });
    },
    [authors, emojiData],
  );

  return [state, setState, update];
}

export type InsertElement =
  | {
      type: "emoji";
      emojiId: string;
      native: string | null;
      aturi?: string | null;
      cid?: string | null;
      imageUrl?: string | null;
      text: string;
      seq: number;
    }
  | { type: "mention"; handle: string; did: string | null; seq: number }
  | { type: "text"; text: string; seq: number }
  | { type: "clear"; seq: number };

interface PluginsProps {
  authors: AuthorEntry[];
  emojiData: EmojiData | null;
  skinTone: number;
  onSubmit: (msg: RichTextResult) => void;
  insertElement?: InsertElement;
  onMentionQuery?: (query: string | null) => void;
  onEmojiQuery?: (query: string | null) => void;
  onEnter?: (msg: RichTextResult) => void;
}

function Plugins({
  authors,
  emojiData,
  skinTone,
  onSubmit,
  insertElement,
  onMentionQuery,
  onEmojiQuery,
  onEnter,
}: PluginsProps) {
  const [editor] = useLexicalComposerContext();
  const [suggestions, setSuggestions, updateSuggestions] = useSuggestions(
    authors,
    emojiData,
  );
  const suggestionsRef = useRef(suggestions);
  suggestionsRef.current = suggestions;

  const hasAnySuggestions =
    suggestions.filteredAuthors.length > 0 ||
    suggestions.filteredEmojis.length > 0;

  const insertMention = useCallback(
    (author: AuthorEntry) => {
      editor.update(() => {
        const selection = $getSelection();
        if (!$isRangeSelection(selection)) return;
        const anchor = selection.anchor;
        const node = anchor.getNode();
        if (!$isTextNode(node)) return;
        const text = node.getTextContent();
        const atIdx = text.lastIndexOf("@");
        if (atIdx === -1) return;
        const before = text.slice(0, atIdx);
        const after = text.slice(anchor.offset);
        node.setTextContent(before + after);
        const mentionNode = $createMentionNode(
          author.handle,
          author.did ?? null,
        );
        const spaceNode = new TextNode(" ");
        node.replace(mentionNode);
        mentionNode.insertAfter(spaceNode);
        spaceNode.select();
      });
      setSuggestions((s) => ({
        ...s,
        mentionQuery: null,
        filteredAuthors: [],
      }));
    },
    [editor, setSuggestions],
  );

  const insertEmoji = useCallback(
    (emoji: EmojiEntry) => {
      const native = (emoji.s[skinTone] ?? emoji.s[0])?.n ?? null;
      const text = native ?? `:${emoji.id}: `;
      editor.update(() => {
        const selection = $getSelection();
        if (!$isRangeSelection(selection)) return;
        const anchor = selection.anchor;
        const node = anchor.getNode();
        if (!$isTextNode(node)) return;
        const raw = node.getTextContent();
        const colonIdx = raw.lastIndexOf(":");
        if (colonIdx === -1) return;
        const before = raw.slice(0, colonIdx);
        const after = raw.slice(anchor.offset);
        node.setTextContent(before + after);
        const emojiNode = $createEmojiNode(text.trimEnd(), emoji.id, native);
        const spaceNode = new TextNode(" ");
        node.replace(emojiNode);
        emojiNode.insertAfter(spaceNode);
        spaceNode.select();
      });
      setSuggestions((s) => ({ ...s, emojiQuery: null, filteredEmojis: [] }));
    },
    [editor, skinTone, setSuggestions],
  );

  const onMentionQueryRef = useRef(onMentionQuery);
  onMentionQueryRef.current = onMentionQuery;
  const onEmojiQueryRef = useRef(onEmojiQuery);
  onEmojiQueryRef.current = onEmojiQuery;
  const updateSuggestionsRef = useRef(updateSuggestions);
  updateSuggestionsRef.current = updateSuggestions;

  // Update suggestions on every editor change — registered once, reads latest callbacks via refs
  useEffect(() => {
    return editor.registerUpdateListener(() => {
      editor.read(() => {
        const textBefore = getTextBeforeCursor();

        if (onMentionQueryRef.current || onEmojiQueryRef.current) {
          // Native path: emit queries upward, clear internal state
          const atIdx = textBefore.lastIndexOf("@");
          const colonIdx = textBefore.lastIndexOf(":");
          if (atIdx !== -1 && (colonIdx === -1 || atIdx > colonIdx)) {
            const q = textBefore.slice(atIdx + 1).toLowerCase();
            if (!q.includes(" ")) {
              onMentionQueryRef.current?.(q);
              onEmojiQueryRef.current?.(null);
            } else {
              onMentionQueryRef.current?.(null);
              onEmojiQueryRef.current?.(null);
            }
          } else if (colonIdx !== -1) {
            const q = textBefore.slice(colonIdx + 1).toLowerCase();
            if (q.length >= 3 && !q.includes(" ")) {
              onMentionQueryRef.current?.(null);
              onEmojiQueryRef.current?.(q);
            } else {
              onMentionQueryRef.current?.(null);
              onEmojiQueryRef.current?.(null);
            }
          } else {
            onMentionQueryRef.current?.(null);
            onEmojiQueryRef.current?.(null);
          }
        } else {
          updateSuggestionsRef.current(textBefore);
        }
      });
    });
  }, [editor]);

  const lastAppliedSeqRef = useRef<number>(-1);

  // External insert / clear — deduplicated by seq so bridge re-renders don't replay
  useEffect(() => {
    if (!insertElement) return;
    if (insertElement.seq === lastAppliedSeqRef.current) return;
    console.log("Applying external insert", insertElement);
    lastAppliedSeqRef.current = insertElement.seq;
    editor.update(() => {
      if (insertElement.type === "clear") {
        $getRoot().clear();
        return;
      }
      const selection = $getSelection();
      let targetSelection = $isRangeSelection(selection) ? selection : null;
      if (!targetSelection) {
        $getRoot().selectEnd();
        const sel = $getSelection();
        if ($isRangeSelection(sel)) targetSelection = sel;
      }
      if (!targetSelection) return;
      if (insertElement.type === "emoji") {
        const emojiNode = $createEmojiNode(
          insertElement.text,
          insertElement.emojiId,
          insertElement.native,
          insertElement.aturi ?? null,
          insertElement.cid ?? null,
          insertElement.imageUrl ?? null,
        );
        const spaceNode = new TextNode(" ");
        const anchor = targetSelection.anchor;
        const anchorNode = anchor.getNode();
        if ($isTextNode(anchorNode)) {
          const raw = anchorNode.getTextContent();
          const colonIdx = raw.lastIndexOf(":");
          if (colonIdx !== -1) {
            const before = raw.slice(0, colonIdx);
            const after = raw.slice(anchor.offset);
            anchorNode.setTextContent(before + after);
            anchorNode.replace(emojiNode);
            emojiNode.insertAfter(spaceNode);
            spaceNode.select();
          } else {
            targetSelection.insertNodes([emojiNode, spaceNode]);
            spaceNode.select();
          }
        } else {
          targetSelection.insertNodes([emojiNode, spaceNode]);
          spaceNode.select();
        }
      } else if (insertElement.type === "mention") {
        const anchor = targetSelection.anchor;
        const node = anchor.getNode();
        if ($isTextNode(node)) {
          const text = node.getTextContent();
          const atIdx = text.lastIndexOf("@");
          const before =
            atIdx !== -1 ? text.slice(0, atIdx) : text.slice(0, anchor.offset);
          const after = text.slice(anchor.offset);
          node.setTextContent(before + after);
        }
        const mentionNode = $createMentionNode(
          insertElement.handle,
          insertElement.did,
        );
        const spaceNode = new TextNode(" ");
        const anchorNode = targetSelection.anchor.getNode();
        if ($isTextNode(anchorNode)) {
          anchorNode.replace(mentionNode);
        } else {
          targetSelection.insertNodes([mentionNode]);
        }
        mentionNode.insertAfter(spaceNode);
        spaceNode.select();
      } else {
        targetSelection.insertText(insertElement.text);
      }
    });
  }, [editor, insertElement]);

  // Keep a ref to onEnter so the keyboard handler doesn't re-register on every render
  const onEnterRef = useRef(onEnter);
  onEnterRef.current = onEnter;

  // Keyboard navigation
  useEffect(() => {
    const hasSuggestions = () => {
      const s = suggestionsRef.current;
      return s.filteredAuthors.length > 0 || s.filteredEmojis.length > 0;
    };

    const removeEnter = editor.registerCommand(
      KEY_ENTER_COMMAND,
      (event) => {
        if (event?.shiftKey) return false;
        event?.preventDefault();
        const s = suggestionsRef.current;
        if (s.filteredAuthors.length > 0) {
          insertMention(
            s.filteredAuthors[s.highlightedIndex] ?? s.filteredAuthors[0],
          );
          return true;
        }
        if (s.filteredEmojis.length > 0) {
          insertEmoji(
            s.filteredEmojis[s.highlightedIndex] ?? s.filteredEmojis[0],
          );
          return true;
        }
        const msg = extractRichText(editor);
        if (onEnterRef.current) {
          // Native path: let native side decide submit vs confirm suggestion.
          // Native controls editor clear via insertElement={type:"clear"}.
          onEnterRef.current(msg);
        } else if (msg.text) {
          onSubmit(msg);
          editor.update(() => {
            $getRoot().clear();
          });
        }
        return true;
      },
      COMMAND_PRIORITY_HIGH,
    );

    const removeTab = editor.registerCommand(
      KEY_TAB_COMMAND,
      (event) => {
        const s = suggestionsRef.current;
        if (s.filteredAuthors.length > 0) {
          event?.preventDefault();
          insertMention(
            s.filteredAuthors[s.highlightedIndex] ?? s.filteredAuthors[0],
          );
          return true;
        }
        if (s.filteredEmojis.length > 0) {
          event?.preventDefault();
          insertEmoji(
            s.filteredEmojis[s.highlightedIndex] ?? s.filteredEmojis[0],
          );
          return true;
        }
        return false;
      },
      COMMAND_PRIORITY_HIGH,
    );

    const removeUp = editor.registerCommand(
      KEY_ARROW_UP_COMMAND,
      (event) => {
        const s = suggestionsRef.current;
        if (hasSuggestions()) {
          event?.preventDefault();
          setSuggestions((prev) => ({
            ...prev,
            highlightedIndex: Math.max(prev.highlightedIndex - 1, 0),
          }));
          return true;
        }
        return false;
      },
      COMMAND_PRIORITY_HIGH,
    );

    const removeDown = editor.registerCommand(
      KEY_ARROW_DOWN_COMMAND,
      (event) => {
        const s = suggestionsRef.current;
        const maxIdx =
          s.filteredAuthors.length > 0
            ? s.filteredAuthors.length - 1
            : s.filteredEmojis.length - 1;
        if (maxIdx >= 0) {
          event?.preventDefault();
          setSuggestions((prev) => ({
            ...prev,
            highlightedIndex: Math.min(prev.highlightedIndex + 1, maxIdx),
          }));
          return true;
        }
        return false;
      },
      COMMAND_PRIORITY_HIGH,
    );

    const removeEscape = editor.registerCommand(
      KEY_ESCAPE_COMMAND,
      (_event) => {
        if (hasSuggestions()) {
          setSuggestions((prev) => ({
            ...prev,
            mentionQuery: null,
            emojiQuery: null,
            filteredAuthors: [],
            filteredEmojis: [],
          }));
          return true;
        }
        return false;
      },
      COMMAND_PRIORITY_HIGH,
    );

    return () => {
      removeEnter();
      removeTab();
      removeUp();
      removeDown();
      removeEscape();
    };
  }, [editor, insertMention, insertEmoji, onSubmit, setSuggestions]);

  if (!hasAnySuggestions || onMentionQuery || onEmojiQuery) return null;

  return (
    <div
      style={{
        position: "absolute",
        bottom: "100%",
        left: 0,
        right: 0,
        marginBottom: 6,
        background: "#1f2937",
        borderRadius: 8,
        boxShadow: "0 4px 12px rgba(0,0,0,0.4)",
        maxHeight: 200,
        overflowY: "auto",
        zIndex: 50,
      }}
    >
      {suggestions.filteredAuthors.length > 0
        ? suggestions.filteredAuthors.map((author, i) => (
            <div
              key={author.handle}
              onMouseDown={(e) => {
                e.preventDefault();
                insertMention(author);
              }}
              style={{
                padding: "8px 12px",
                cursor: "pointer",
                background:
                  i === suggestions.highlightedIndex
                    ? "rgba(255,255,255,0.1)"
                    : "transparent",
                fontWeight: 700,
                color: author.color
                  ? `rgb(${author.color.red},${author.color.green},${author.color.blue})`
                  : "#a5b4fc",
                fontSize: 14,
              }}
            >
              @{author.handle}
            </div>
          ))
        : suggestions.filteredEmojis.map((emoji, i) => {
            const native = (emoji.s[skinTone] ?? emoji.s[0])?.n ?? "";
            return (
              <div
                key={emoji.id}
                onMouseDown={(e) => {
                  e.preventDefault();
                  insertEmoji(emoji);
                }}
                style={{
                  padding: "6px 12px",
                  cursor: "pointer",
                  display: "flex",
                  alignItems: "center",
                  gap: 10,
                  background:
                    i === suggestions.highlightedIndex
                      ? "rgba(255,255,255,0.1)"
                      : "transparent",
                }}
              >
                <span style={{ fontSize: 18, lineHeight: 1 }}>{native}</span>
                <span style={{ color: "white", fontSize: 13 }}>
                  <code
                    style={{
                      background: "rgba(0,0,0,0.4)",
                      borderRadius: 3,
                      padding: "1px 4px",
                      fontSize: 12,
                    }}
                  >
                    :{emoji.id}:
                  </code>{" "}
                  {emoji.m}
                </span>
              </div>
            );
          })}
    </div>
  );
}

// ── Editor config ────────────────────────────────────────────────────────────

const EDITOR_THEME = {
  paragraph: "editor-paragraph",
};

// ── Root component ───────────────────────────────────────────────────────────

export default function ChatEditor({
  authors,
  emojiData,
  skinTone,
  placeholder = "Type a message...",
  onSubmit,
  insertElement,
  onDOMLayout,
  onMentionQuery,
  onEmojiQuery,
  onEnter,
}: ChatEditorProps) {
  useSize(onDOMLayout);
  const config: InitialConfigType = {
    namespace: "ChatEditor",
    theme: EDITOR_THEME,
    nodes: [MentionNode, EmojiNode],
    onError: (error) => console.error("[ChatEditor]", error),
  };

  return (
    <div style={{ position: "relative", width: "100%", minHeight: 43 }}>
      <style>{`
        html, body {
          min-height: 43px;
          margin: 0;
          padding: 0;
        }
        .editor-root {
          background: #111827;
          border-radius: 12px;
          border: 1px solid #374151;
          padding: 10px 14px;
          color: white;
          font-size: 14px;
          font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
          line-height: 1.5;
          min-height: 43px;
          outline: none;
          white-space: pre-wrap;
          word-break: break-word;
          cursor: text;
        }
        .editor-root:focus {
          border-color: #6366f1;
        }
        .editor-placeholder {
          position: absolute;
          top: 10px;
          left: 14px;
          color: #6b7280;
          font-size: 14px;
          font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
          pointer-events: none;
          user-select: none;
        }
      `}</style>
      <LexicalComposer initialConfig={config}>
        <PlainTextPlugin
          contentEditable={<ContentEditable className="editor-root" />}
          placeholder={<div className="editor-placeholder">{placeholder}</div>}
          ErrorBoundary={LexicalErrorBoundary}
        />
        <HistoryPlugin />
        <Plugins
          authors={authors}
          emojiData={emojiData}
          skinTone={skinTone}
          onSubmit={onSubmit}
          insertElement={insertElement}
          onMentionQuery={onMentionQuery}
          onEmojiQuery={onEmojiQuery}
          onEnter={onEnter}
        />
      </LexicalComposer>
    </div>
  );
}
