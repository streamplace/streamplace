import type { LivestreamStore } from "@streamplace/core";
import { Extension } from "@tiptap/core";
import MentionBase from "@tiptap/extension-mention";
import Placeholder from "@tiptap/extension-placeholder";
import { PluginKey } from "@tiptap/pm/state";
import { EditorContent, ReactRenderer, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { exitSuggestion, Suggestion } from "@tiptap/suggestion";
import type { SkinTone } from "frimousse";
import { Reply, Smile, X } from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { ChatMessageViewHydrated } from "streamplace";
import { useStore } from "zustand";
import { useChatSend } from "../../hooks/use-chat-send";
import { skinToneIndex, useSkinTone } from "../../hooks/use-skin-tone";
import {
  getEmojiData,
  getSkinNative,
  searchEmojis,
  useEmojiData,
  type Emoji,
  type EmojiData,
} from "../../lib/emoji-data";
import { useSession } from "../../lib/session";
import { useStore as useAppStore } from "../../lib/store";
import { Button } from "../ui/button";
import { EmojiList } from "./emoji-list";
import { EmojiPicker } from "./emoji-picker";
import { MentionList, type MentionItem } from "./mention-list";

const MAX_LENGTH = 300;

// Two distinct plugin keys so the mention and emoji suggestion plugins can
// coexist and be queried independently from `handleKeyDown` and elsewhere.
const MentionPluginKey = new PluginKey("streamplace-mention");
const EmojiPluginKey = new PluginKey("streamplace-emoji");

// Extend the default Mention node to store our custom attributes (color, avatar,
// did, handle, displayName) so they survive serialization inside the editor.
const Mention = MentionBase.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      did: { default: null },
      handle: { default: null },
      displayName: { default: null },
      avatar: { default: null },
      color: { default: null },
    };
  },
});

// Module-level refs so the suggestion items functions read from fresh data
// without recreating the editor (or the suggestion plugin) on every render.
let mentionDataRef: { allItems: MentionItem[] } = { allItems: [] };
let emojiDataRef: { data: EmojiData | null } = { data: null };

function getMentionSuggestions(
  chat: ChatMessageViewHydrated[],
  authors: { [key: string]: ChatMessageViewHydrated["chatProfile"] },
): MentionItem[] {
  const seen = new Set<string>();
  const items: MentionItem[] = [];
  for (const msg of chat) {
    const did = msg.author.did;
    if (seen.has(did)) continue;
    seen.add(did);
    const profile = authors[did];
    items.push({
      did,
      handle: msg.author.handle || "",
      displayName: msg.author.displayName || "",
      avatar: msg.author.avatar || null,
      color: profile?.color || null,
    });
  }
  return items;
}

function createMentionSuggestion() {
  return {
    pluginKey: MentionPluginKey,
    char: "@",
    items: ({ query }: { query: string }) => {
      const q = query.toLowerCase();
      return mentionDataRef.allItems.filter(
        (item) =>
          item.handle.toLowerCase().includes(q) ||
          item.displayName.toLowerCase().includes(q),
      );
    },
    render: () => {
      let component: ReactRenderer<any> | null = null;
      let popup: HTMLDivElement | null = null;

      return {
        onStart: (props: any) => {
          component = new ReactRenderer(MentionList, {
            props: {
              items: props.items,
              command: (item: MentionItem) => {
                props.command({
                  id: item.handle,
                  label: `@${item.handle}`,
                  did: item.did,
                  handle: item.handle,
                  displayName: item.displayName,
                  avatar: item.avatar,
                  color: item.color,
                });
              },
            },
            editor: props.editor,
          });

          if (!props.clientRect) return;

          popup = document.createElement("div");
          popup.style.position = "absolute";
          popup.style.zIndex = "50";
          // Match the editor's width so the suggestion list spans the
          // full chat input column instead of a narrow 220px column.
          popup.style.width = `${props.editor.view.dom.offsetWidth}px`;
          popup.appendChild(component.element);
          document.body.appendChild(popup);

          const rect = props.clientRect();
          if (rect) {
            popup.style.left = `${rect.left}px`;
            popup.style.bottom = `${window.innerHeight - rect.top + 4}px`;
          }
        },
        onUpdate: (props: any) => {
          component?.updateProps({
            items: props.items,
            command: (item: MentionItem) => {
              props.command({
                id: item.handle,
                label: `@${item.handle}`,
                did: item.did,
                handle: item.handle,
                displayName: item.displayName,
                avatar: item.avatar,
                color: item.color,
              });
            },
          });

          if (!props.clientRect) return;

          const rect = props.clientRect();
          if (rect && popup) {
            popup.style.left = `${rect.left}px`;
            popup.style.bottom = `${window.innerHeight - rect.top + 4}px`;
            popup.style.width = `${props.editor.view.dom.offsetWidth}px`;
          }
        },
        onKeyDown: (props: any) => {
          if (props.event.key === "Escape") {
            destroy();
            return true;
          }
          return component?.ref?.onKeyDown?.(props) ?? false;
        },
        onExit: () => {
          destroy();
        },
      };

      function destroy() {
        component?.destroy();
        popup?.remove();
        component = null;
        popup = null;
      }
    },
  };
}

// Render-factory for suggestion popups that take an arbitrary React list
// component. Centralizes the DOM-popup pattern (anchor at the suggestion's
// clientRect, append to body, reposition on scroll/update, destroy on exit)
// so the mention and emoji suggestions can share the choreography.
interface PopupProps<T> {
  items: T[];
  command: (item: T) => void;
  clientRect?: (() => DOMRect | null) | null;
  editor: any;
}

interface PopupRenderHandlers<T> {
  onStart: (props: PopupProps<T>) => void;
  onUpdate: (props: PopupProps<T>) => void;
  onKeyDown: (props: { event: KeyboardEvent }) => boolean;
  onExit: () => void;
}

function createBodyPopup<T>(
  ListComponent: React.ComponentType<any>,
  buildCommand: (props: PopupProps<T>) => (item: T) => void,
) {
  return (): PopupRenderHandlers<T> => {
    let component: ReactRenderer<any> | null = null;
    let popup: HTMLDivElement | null = null;

    return {
      onStart(props) {
        component = new ReactRenderer(ListComponent, {
          props: {
            items: props.items,
            command: buildCommand(props),
          },
          editor: props.editor,
        });
        if (!props.clientRect) return;

        popup = document.createElement("div");
        popup.style.position = "absolute";
        popup.style.zIndex = "50";
        popup.style.width = `${props.editor.view.dom.offsetWidth}px`;
        popup.appendChild(component.element);
        document.body.appendChild(popup);

        const rect = props.clientRect();
        if (rect) {
          popup.style.left = `${rect.left}px`;
          popup.style.bottom = `${window.innerHeight - rect.top + 4}px`;
        }
      },
      onUpdate(props) {
        component?.updateProps({
          items: props.items,
          command: buildCommand(props),
        });
        if (!props.clientRect) return;
        const rect = props.clientRect();
        if (rect && popup) {
          popup.style.left = `${rect.left}px`;
          popup.style.bottom = `${window.innerHeight - rect.top + 4}px`;
          popup.style.width = `${props.editor.view.dom.offsetWidth}px`;
        }
      },
      onKeyDown(props) {
        if (props.event.key === "Escape") {
          component?.destroy();
          popup?.remove();
          component = null;
          popup = null;
          return true;
        }
        return component?.ref?.onKeyDown?.(props) ?? false;
      },
      onExit() {
        component?.destroy();
        popup?.remove();
        component = null;
        popup = null;
      },
    };
  };
}

interface EmojiSuggestionProps {
  getSkinTone: () => SkinTone;
}

function createEmojiSuggestion({ getSkinTone }: EmojiSuggestionProps) {
  return {
    pluginKey: EmojiPluginKey,
    char: ":",
    // Don't pop the popup open on a single `:`; only once the user has
    // typed enough characters to plausibly match something. Without this
    // gate the popup opens with an empty list and looks broken.
    shouldShow: ({ query }: { query: string }) => query.length >= 3,
    items: ({ query }: { query: string }) => {
      const data = emojiDataRef.data;
      if (data) return searchEmojis(data, query);
      // Data not yet loaded; kick off the fetch, cache the result, and
      // resolve the items list once it lands. The suggestion plugin
      // accepts a Promise return value and will re-fire onUpdate.
      return getEmojiData().then((d) => {
        emojiDataRef.data = d;
        return searchEmojis(d, query);
      });
    },
    // Insert the native char with the user's selected skin tone. We
    // append a space so the cursor lands past the emoji and the user
    // can keep typing without an extra keystroke.
    command: ({ editor, range, props }: any) => {
      const native = getSkinNative(props, skinToneIndex(getSkinTone()));
      editor
        .chain()
        .focus()
        .deleteRange(range)
        .insertContent(native + " ")
        .run();
    },
    render: createBodyPopup<Emoji>(
      EmojiList,
      (props) => (item) => props.command(item),
    ),
  };
}

export function ChatInput({ store }: { store: LivestreamStore }) {
  const { t } = useTranslation("common");
  const { state } = useSession();
  const isAuthed = state.status === "authenticated";
  const send = useChatSend(store);
  const replyToMessage = useStore(store, (s) => s.replyToMessage);
  const chat = useStore(store, (s) => s.chat);
  const authors = useStore(store, (s) => s.authors);
  const [error, setError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const [emojiOpen, setEmojiOpen] = useState(false);
  const [skinTone, setSkinTone] = useSkinTone();

  // Keep module-level refs fresh so the suggestion items functions always
  // have up-to-date data without recreating the editor.
  mentionDataRef.allItems = getMentionSuggestions(chat, authors);
  const emojiData = useEmojiData();
  emojiDataRef.data = emojiData;

  // skinToneRef lets the suggestion command (a closure created once) read
  // the latest skin tone value without recreating the suggestion plugin.
  const skinToneRef = useRef(skinTone);
  skinToneRef.current = skinTone;

  const mentionConfig = useMemo(() => createMentionSuggestion(), []);
  const emojiConfig = useMemo(
    () => createEmojiSuggestion({ getSkinTone: () => skinToneRef.current }),
    [],
  );

  // Store onSubmit in a ref so editorProps can access it without depending on
  // sending state (which would recreate the editor on every send).
  const onSubmitRef = useRef<() => void>(() => {});

  // Ref to the smile trigger button; passed to the picker so it can
  // position itself relative to it (the picker portals to document.body
  // to escape the chat sidebar's overflow-hidden wrapper).
  const smileButtonRef = useRef<HTMLButtonElement>(null);

  const editor = useEditor(
    {
      extensions: [
        StarterKit.configure({
          heading: false,
          bold: false,
          italic: false,
          strike: false,
          code: false,
          blockquote: false,
          bulletList: false,
          orderedList: false,
          codeBlock: false,
          horizontalRule: false,
          hardBreak: false,
        }),
        Placeholder.configure({
          placeholder: t("chat-send-message"),
        }),
        Mention.configure({
          HTMLAttributes: {
            class: "mention",
          },
          suggestion: mentionConfig,
          renderText({ node }) {
            return node.attrs.label ?? `@${node.attrs.id}`;
          },
          renderHTML({ node }) {
            const id = node.attrs.id as string;
            const label = (node.attrs.label as string) ?? `@${id}`;
            const color = node.attrs.color as {
              red: number;
              green: number;
              blue: number;
            } | null;
            const colorStr = color
              ? `rgb(${color.red}, ${color.green}, ${color.blue})`
              : undefined;
            return [
              "span",
              {
                "data-type": "mention",
                "data-id": id,
                "data-label": label,
                class: "mention",
                style: colorStr ? `color: ${colorStr}` : undefined,
              },
              label,
            ];
          },
        }),
        // The emoji suggestion plugin lives on a thin extension that just
        // registers the ProseMirror plugin. We insert the native character
        // as plain text (no custom node needed) and rely on ProseMirror's
        // text handling for cursor/backspace behavior.
        Extension.create({
          name: "emoji-suggestion",
          addProseMirrorPlugins() {
            return [
              Suggestion({
                editor: this.editor,
                ...emojiConfig,
              }),
            ];
          },
        }),
      ],
      editorProps: {
        attributes: {
          // ProseMirror's default CSS adds white-space: pre-wrap and
          // word-wrap: break-word, but that stylesheet isn't imported
          // here, so we add the equivalents via Tailwind to keep long
          // input from overflowing horizontally instead of wrapping.
          class:
            "min-h-[36px] max-h-[120px] min-w-0 overflow-y-auto whitespace-pre-wrap break-words px-3 py-2 text-sm outline-none",
        },
        handleKeyDown: (view, event) => {
          // Submit on Enter (not Shift+Enter) unless a suggestion popup is
          // active. ProseMirror dispatches handleKeyDown with editor-view
          // props first and plugins after, so we can't rely on the
          // suggestion plugin short-circuiting us; we have to check both
          // plugin states and decline to handle the event ourselves so
          // whichever plugin is open still gets a turn.
          if (event.key === "Enter" && !event.shiftKey) {
            const mentionActive = MentionPluginKey.getState(view.state)?.active;
            const emojiActive = EmojiPluginKey.getState(view.state)?.active;
            if (mentionActive || emojiActive) return false;
            onSubmitRef.current();
            return true;
          }
          // Escape dismisses whichever popup is open (whichever the user
          // most recently engaged with), even when the suggestion plugin
          // is still processing.
          if (event.key === "Escape") {
            const mentionActive = MentionPluginKey.getState(view.state)?.active;
            const emojiActive = EmojiPluginKey.getState(view.state)?.active;
            if (mentionActive) {
              exitSuggestion(view, MentionPluginKey);
              return true;
            }
            if (emojiActive) {
              exitSuggestion(view, EmojiPluginKey);
              return true;
            }
          }
          return false;
        },
      },
      onUpdate: () => {
        setError(null);
      },
    },
    [],
  );

  const onSubmit = useCallback(async () => {
    if (!editor || sending) return;
    const text = editor.getText().trim();
    if (!text) return;

    setError(null);
    setSending(true);
    try {
      await send(text);
      editor.commands.clearContent();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("chat-failed-send"));
    } finally {
      setSending(false);
    }
  }, [editor, sending, send]);

  // Keep the ref fresh so editorProps.handleKeyDown can call the latest onSubmit.
  onSubmitRef.current = () => onSubmit();

  const handleEmojiSelect = useCallback(
    (
      emoji:
        | { type: "standard"; native: string }
        | { type: "custom"; name: string },
    ) => {
      if (editor) {
        const text =
          emoji.type === "standard" ? emoji.native : `:${emoji.name}: `;
        editor.chain().focus().insertContent(text).run();
      }
      setEmojiOpen(false);
    },
    [editor],
  );

  if (!isAuthed) {
    return (
      <div className="py-2 text-center text-sm text-(--color-fg-muted)">
        <Button
          type="button"
          variant="muted"
          onClick={() => useAppStore.getState().openLoginModal()}
          className="text-lg font-medium text-(--color-accent) hover:underline"
        >
          {t("chat-log-in-to")}
        </Button>
      </div>
    );
  }

  const textLength = editor?.getText().length ?? 0;

  return (
    <div>
      {replyToMessage && (
        <div className="mb-1 flex items-center gap-2 rounded border border-(--color-border) bg-(--color-bg-overlay) px-2 py-1 text-xs">
          <Reply className="h-3 w-3 shrink-0 text-(--color-fg-muted)" />
          <span className="flex-1 truncate text-(--color-fg-muted)">
            {t("chat-replying-to", {
              handle: replyToMessage.author.handle || replyToMessage.author.did,
            })}
          </span>
          <button
            type="button"
            onClick={() =>
              store.setState((s) => ({ ...s, replyToMessage: null }))
            }
            className="text-(--color-fg-muted) hover:text-(--color-fg)"
          >
            <X className="h-3 w-3" />
          </button>
        </div>
      )}

      <form
        onSubmit={(e) => {
          e.preventDefault();
          onSubmit();
        }}
        className="flex items-end gap-2"
      >
        <div className="relative min-w-0 flex-1 rounded-lg border border-(--color-border) bg-(--color-bg) focus-within:border-(--color-accent)">
          {editor && <EditorContent editor={editor} />}

          {textLength > 200 && (
            <span
              className={`pointer-events-none absolute top-1/2 right-2 -translate-y-1/2 text-[10px] tabular-nums ${textLength >= MAX_LENGTH ? "text-(--color-danger)" : "text-(--color-fg-subtle)"}`}
            >
              {MAX_LENGTH - textLength}
            </span>
          )}
        </div>

        <Button
          ref={smileButtonRef}
          type="button"
          variant="ghost"
          size="icon"
          className="h-9 w-9 shrink-0"
          aria-label={t("chat-insert-emoji")}
          onClick={() => setEmojiOpen((o) => !o)}
        >
          <Smile className="h-4 w-4" />
        </Button>

        <Button type="submit" size="lg" disabled={sending || textLength === 0}>
          {sending ? "..." : t("chat-send-button")}
        </Button>
      </form>

      {emojiOpen && (
        <EmojiPicker
          isOpen={emojiOpen}
          onClose={() => setEmojiOpen(false)}
          onSelect={handleEmojiSelect}
          skinTone={skinTone}
          onSkinToneChange={setSkinTone}
          anchorRef={smileButtonRef}
        />
      )}

      {error && (
        <div className="mt-1 text-xs text-(--color-danger)">{error}</div>
      )}
    </div>
  );
}
