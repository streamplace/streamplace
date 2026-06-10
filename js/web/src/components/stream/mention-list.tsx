import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from "react";

export interface MentionItem {
  did: string;
  handle: string;
  displayName: string;
  avatar: string | null;
  color: { red: number; green: number; blue: number } | null;
}

export interface MentionListRef {
  onKeyDown: (props: { event: KeyboardEvent }) => boolean;
}

interface MentionListProps {
  items: MentionItem[];
  command: (item: MentionItem) => void;
}

export const MentionList = forwardRef<MentionListRef, MentionListProps>(
  function MentionListImpl({ items, command }, ref) {
    const [selectedIndex, setSelectedIndex] = useState(0);
    const listRef = useRef<HTMLDivElement>(null);

    // Refs always point at the latest props/state. The imperative handle
    // reads from these so it never closes over a stale selectedIndex or
    // command — that was the cause of Enter/Tab inserting the wrong item
    // (or nothing) when the user navigated with arrow keys and then
    // committed before the next ref update landed.
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
      if (item) {
        item.scrollIntoView({ block: "nearest" });
      }
    }, []);

    // Side effect (scroll) lives in an effect, not inside the setState
    // updater — the updater can be called more than once under StrictMode
    // or concurrent rendering.
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
            setSelectedIndex((i) => {
              const next = (i - 1 + currentItems.length) % currentItems.length;
              return next;
            });
            return true;
          }
          if (event.key === "ArrowDown") {
            setSelectedIndex((i) => {
              const next = (i + 1) % currentItems.length;
              return next;
            });
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
        className="bg-[var(--color-bg-elevated)] border border-[var(--color-border)] rounded-lg shadow-lg overflow-hidden max-h-64 overflow-y-auto"
      >
        {items.map((item, i) => {
          const colorStr = item.color
            ? `rgb(${item.color.red}, ${item.color.green}, ${item.color.blue})`
            : undefined;
          const isSelected = i === selectedIndex;
          return (
            <button
              key={item.did}
              type="button"
              data-selected={isSelected}
              role="option"
              aria-selected={isSelected}
              className={`flex items-center gap-2 w-full px-3 py-2 text-left text-sm hover:bg-[var(--color-bg-overlay)] focus:bg-[var(--color-bg-overlay)] outline-none ${isSelected ? "bg-[var(--color-bg-overlay)]" : ""}`}
              // mousedown (not click) keeps the editor focused so the
              // suggestion plugin stays alive; click fires after blur and
              // would close the popup before command() runs.
              onMouseDown={(e) => {
                e.preventDefault();
                if (item) command(item);
              }}
              onMouseEnter={() => setSelectedIndex(i)}
            >
              {item.avatar ? (
                <img
                  src={item.avatar}
                  alt=""
                  className="w-6 h-6 rounded-full flex-shrink-0"
                  onError={(e) => {
                    (e.currentTarget as HTMLImageElement).style.display =
                      "none";
                  }}
                />
              ) : (
                <div className="w-6 h-6 rounded-full bg-[var(--color-muted)] flex-shrink-0" />
              )}
              <span
                className="font-medium text-sm truncate"
                style={colorStr ? { color: colorStr } : undefined}
              >
                {item.displayName || item.handle}
              </span>
              {item.displayName && item.handle && (
                <span className="text-xs text-[var(--color-fg-muted)] truncate">
                  @{item.handle}
                </span>
              )}
            </button>
          );
        })}
      </div>
    );
  },
);
