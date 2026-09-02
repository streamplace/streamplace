import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from "react";
import type { Emoji } from "../../lib/emoji-data";

export interface EmojiListRef {
  onKeyDown: (props: { event: KeyboardEvent }) => boolean;
}

interface EmojiListProps {
  items: Emoji[];
  command: (item: Emoji) => void;
}

export const EmojiList = forwardRef<EmojiListRef, EmojiListProps>(
  function EmojiListImpl({ items, command }, ref) {
    const [selectedIndex, setSelectedIndex] = useState(0);
    const listRef = useRef<HTMLDivElement>(null);

    // Refs always point at the latest props/state. The imperative handle
    // reads from these so it never closes over a stale selectedIndex or
    // command when the user navigates and commits in the same frame.
    const selectedIndexRef = useRef(0);
    const itemsRef = useRef(items);
    const commandRef = useRef(command);
    useEffect(() => {
      selectedIndexRef.current = selectedIndex;
    }, [selectedIndex]);
    useEffect(() => {
      itemsRef.current = items;
    }, [items]);
    useEffect(() => {
      commandRef.current = command;
    }, [command]);

    useEffect(() => {
      setSelectedIndex(0);
    }, [items]);

    const scrollToIndex = useCallback((index: number) => {
      const list = listRef.current;
      if (!list) return;
      const item = list.children[index] as HTMLElement;
      if (item) item.scrollIntoView({ block: "nearest" });
    }, []);

    useEffect(() => {
      scrollToIndex(selectedIndex);
    }, [selectedIndex, scrollToIndex]);

    useImperativeHandle(
      ref,
      () => ({
        onKeyDown: ({ event }: { event: KeyboardEvent }) => {
          const currentItems = itemsRef.current;
          if (currentItems.length === 0) return false;
          if (event.key === "ArrowUp") {
            setSelectedIndex(
              (i) => (i - 1 + currentItems.length) % currentItems.length,
            );
            return true;
          }
          if (event.key === "ArrowDown") {
            setSelectedIndex((i) => (i + 1) % currentItems.length);
            return true;
          }
          if (event.key === "Enter" || event.key === "Tab") {
            const item = currentItems[selectedIndexRef.current];
            if (item) commandRef.current(item);
            return true;
          }
          return false;
        },
      }),
      [],
    );

    if (items.length === 0) return null;

    return (
      <div
        ref={listRef}
        className="max-h-64 overflow-hidden overflow-y-auto rounded-lg border border-(--color-border) bg-(--color-bg-elevated) shadow-lg"
      >
        {items.map((item, i) => {
          const isSelected = i === selectedIndex;
          const native = item.s[0]?.n;
          return (
            <button
              key={item.id}
              type="button"
              data-selected={isSelected}
              role="option"
              aria-selected={isSelected}
              className={`flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm outline-none hover:bg-(--color-bg-overlay) focus:bg-(--color-bg-overlay) ${isSelected ? "bg-(--color-bg-overlay)" : ""}`}
              // mousedown (not click) keeps the editor focused so the
              // suggestion plugin stays alive; click fires after blur and
              // would close the popup before command() runs.
              onMouseDown={(e) => {
                e.preventDefault();
                command(item);
              }}
              onMouseEnter={() => setSelectedIndex(i)}
            >
              <span
                aria-hidden
                className="flex h-6 w-6 shrink-0 items-center justify-center text-lg leading-none"
              >
                {native}
              </span>
              <span className="truncate text-sm font-medium">{item.m}</span>
              <span className="truncate font-mono text-xs text-(--color-fg-muted)">
                :{item.id}:
              </span>
            </button>
          );
        })}
        <div className="border-t border-(--color-border) px-3 py-1 text-[10px] text-(--color-fg-subtle)">
          ↑↓ navigate · ↵ select · esc dismiss
        </div>
      </div>
    );
  },
);
