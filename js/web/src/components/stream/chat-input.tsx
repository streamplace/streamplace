import type { LivestreamStore } from "@streamplace/core";
import { Link } from "@tanstack/react-router";
import MentionBase from "@tiptap/extension-mention";
import Placeholder from "@tiptap/extension-placeholder";
import { EditorContent, ReactRenderer, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { SuggestionPluginKey } from "@tiptap/suggestion";
import { EmojiPicker } from "frimousse";
import { Reply, Smile, X } from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { ChatMessageViewHydrated } from "streamplace";
import { useStore } from "zustand";
import { useChatSend } from "../../hooks/use-chat-send";
import { EMPTY_LOGIN_SEARCH } from "../../lib/login-search";
import { useSession } from "../../lib/session";
import { Button } from "../ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "../ui/popover";
import { MentionList, type MentionItem } from "./mention-list";

const MAX_LENGTH = 300;

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

// Stable ref for mention data that the suggestion items function reads from.
// Updated on every render so the suggestion always has fresh data without
// recreating the editor.
let mentionDataRef: { allItems: MentionItem[] } = { allItems: [] };

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

  // Keep module-level ref fresh so the suggestion items function always has
  // up-to-date data without recreating the editor.
  mentionDataRef.allItems = getMentionSuggestions(chat, authors);

  const mentionConfig = useMemo(() => createMentionSuggestion(), []);

  // Store onSubmit in a ref so editorProps can access it without depending on
  // sending state (which would recreate the editor on every send).
  const onSubmitRef = useRef<() => void>(() => {});

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
      ],
      editorProps: {
        attributes: {
          class:
            "min-h-[36px] max-h-[120px] overflow-y-auto px-3 py-2 text-sm outline-none",
        },
        handleKeyDown: (view, event) => {
          // Submit on Enter (not Shift+Enter) unless a mention suggestion is
          // active. ProseMirror dispatches handleKeyDown with editor-view
          // props first and plugins after, so we can't rely on the suggestion
          // plugin short-circuiting us — we have to check its state and
          // decline to handle the event ourselves so the plugin still gets a
          // turn.
          if (event.key === "Enter" && !event.shiftKey) {
            const suggestionActive = SuggestionPluginKey.getState(
              view.state,
            )?.active;
            if (suggestionActive) return false;
            onSubmitRef.current();
            return true;
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
    (emoji: { emoji: string }) => {
      if (editor) {
        editor.chain().focus().insertContent(emoji.emoji).run();
      }
      setEmojiOpen(false);
    },
    [editor],
  );

  if (!isAuthed) {
    return (
      <div className="py-1 text-center text-sm text-[var(--color-fg-muted)]">
        <Link
          to="/login"
          search={EMPTY_LOGIN_SEARCH}
          className="font-medium text-[var(--color-accent)] hover:underline"
        >
          {t("log-in")}
        </Link>{" "}
        {t("chat-log-in-to")}
      </div>
    );
  }

  const textLength = editor?.getText().length ?? 0;

  return (
    <div>
      {replyToMessage && (
        <div className="mb-1 flex items-center gap-2 rounded border border-[var(--color-border)] bg-[var(--color-bg-overlay)] px-2 py-1 text-xs">
          <Reply className="h-3 w-3 flex-shrink-0 text-[var(--color-fg-muted)]" />
          <span className="flex-1 truncate text-[var(--color-fg-muted)]">
            {t("chat-replying-to", {
              handle: replyToMessage.author.handle || replyToMessage.author.did,
            })}
          </span>
          <button
            type="button"
            onClick={() =>
              store.setState((s) => ({ ...s, replyToMessage: null }))
            }
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
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
        <div className="relative flex-1 rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] focus-within:border-[var(--color-accent)]">
          {editor && <EditorContent editor={editor} />}

          {textLength > 200 && (
            <span
              className={`pointer-events-none absolute top-1/2 right-2 -translate-y-1/2 text-[10px] tabular-nums ${textLength >= MAX_LENGTH ? "text-[var(--color-danger)]" : "text-[var(--color-fg-subtle)]"}`}
            >
              {MAX_LENGTH - textLength}
            </span>
          )}
        </div>

        <Popover open={emojiOpen} onOpenChange={setEmojiOpen}>
          <PopoverTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-9 w-9 flex-shrink-0"
              aria-label={t("chat-insert-emoji")}
            >
              <Smile className="h-4 w-4" />
            </Button>
          </PopoverTrigger>
          <PopoverContent
            align="end"
            side="top"
            className="h-[320px] w-[280px] overflow-hidden p-0"
          >
            <EmojiPicker.Root onEmojiSelect={handleEmojiSelect}>
              <EmojiPicker.Search className="w-full border-b border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm outline-none" />
              <EmojiPicker.Viewport className="h-[280px] overflow-y-auto">
                <EmojiPicker.List />
              </EmojiPicker.Viewport>
            </EmojiPicker.Root>
          </PopoverContent>
        </Popover>

        <Button type="submit" size="sm" disabled={sending || textLength === 0}>
          {sending ? "..." : t("chat-send-button")}
        </Button>
      </form>

      {error && (
        <div className="mt-1 text-xs text-[var(--color-danger)]">{error}</div>
      )}
    </div>
  );
}
