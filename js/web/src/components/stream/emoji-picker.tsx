// Web-native emoji picker.
//
// The skin tone is lifted to the parent: pass `skinTone` and
// `onSkinToneChange` so the parent's state (and any shortcode-insertion
// logic that reads it) stays in sync with the picker's selection.

import {
  EmojiPicker as FrimousseEmojiPicker,
  SkinTone,
  useSkinTone as useFrimousseSkinTone,
} from "frimousse";
import {
  ChevronUp,
  Flag,
  Hash,
  Lightbulb,
  type LucideIcon,
  PawPrint,
  PersonStanding,
  Pizza,
  Plane,
  Smile,
  Volleyball,
} from "lucide-react";
import type { RefObject } from "react";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import { useEmojiData } from "../../lib/emoji-data";

const CATEGORY_ICONS: { label: string; Icon: LucideIcon }[] = [
  { label: "Smileys & Emotion", Icon: Smile },
  { label: "People & Body", Icon: PersonStanding },
  { label: "Animals & Nature", Icon: PawPrint },
  { label: "Food & Drink", Icon: Pizza },
  { label: "Travel & Places", Icon: Plane },
  { label: "Activities", Icon: Volleyball },
  { label: "Objects", Icon: Lightbulb },
  { label: "Symbols", Icon: Hash },
  { label: "Flags", Icon: Flag },
];

export type SelectedEmoji =
  | { type: "standard"; native: string }
  | { type: "custom"; name: string };

interface EmojiPickerProps {
  isOpen: boolean;
  onClose: () => void;
  onSelect?: (emoji: SelectedEmoji) => void;
  customEmoji?: CustomEmojiEntry[];
  /** Lifted skin tone value (driven by the parent for cross-component sync). */
  skinTone: SkinTone;
  /** Fired when the user picks a new skin tone in the tray. */
  onSkinToneChange?: (tone: SkinTone) => void;
  /**
   * Ref to the trigger element (e.g., the smile button). The picker
   * portals to document.body and positions itself relative to this
   * anchor; the popover that would otherwise host us has an
   * `overflow-hidden` ancestor (the chat sidebar wrapper) that clips
   * any `position: absolute` content above the sidebar's top edge.
   */
  anchorRef: RefObject<HTMLElement | null>;
}

export interface CustomEmojiEntry {
  name: string;
  imageUrl: string;
  alt?: string;
}

interface SkinTonePickerOpen {
  open: boolean;
  setOpen: React.Dispatch<React.SetStateAction<boolean>>;
}

export function SkinToneTray({
  open,
  setOpen,
  skinTone,
  onSelect,
}: SkinTonePickerOpen & {
  skinTone: SkinTone;
  onSelect: (tone: SkinTone) => void;
}) {
  const [, setFrimousseSkinTone, skinToneVariations] =
    useFrimousseSkinTone("👋");

  // Mirror the lifted skinTone into frimousse's internal state so the
  // trigger and tray stay in lockstep with whatever the parent last said.
  useEffect(() => {
    setFrimousseSkinTone(skinTone);
  }, [skinTone, setFrimousseSkinTone]);

  const handleSelect = (tone: SkinTone) => {
    onSelect(tone);
    setOpen(false);
  };

  return (
    <div
      style={{
        overflow: "hidden",
        maxHeight: open ? 48 : 0,
        opacity: open ? 1 : 0,
        transition: "max-height 0.2s ease, opacity 0.15s ease",
        display: "flex",
        gap: 4,
        padding: open ? "6px 8px" : "0 8px",
        borderTop: open ? "1px solid var(--color-border)" : "none",
      }}
    >
      {skinToneVariations.map((variation) => (
        <button
          key={variation.skinTone}
          onClick={() => handleSelect(variation.skinTone)}
          style={{
            width: 30,
            height: 30,
            borderRadius: 6,
            border:
              skinTone === variation.skinTone
                ? "1px solid var(--color-accent)"
                : "1px solid var(--color-border)",
            background:
              skinTone === variation.skinTone
                ? "var(--color-bg-overlay)"
                : "transparent",
            cursor: "pointer",
            fontSize: 18,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          {variation.emoji}
        </button>
      ))}
    </div>
  );
}

export function SkinToneTrigger({
  open,
  setOpen,
  skinTone,
}: SkinTonePickerOpen & { skinTone: SkinTone }) {
  const [, , skinToneVariations] = useFrimousseSkinTone("👋");
  const current =
    skinToneVariations.find((v) => v.skinTone === skinTone) ??
    skinToneVariations[0];

  return (
    <button
      onClick={() => setOpen((o) => !o)}
      title="Skin tone"
      style={{
        display: "flex",
        alignItems: "center",
        gap: 4,
        padding: "2px 4px",
        border: "1px solid var(--color-border)",
        borderRadius: 6,
        background: open ? "var(--color-bg-overlay)" : "transparent",
        cursor: "pointer",
        fontSize: 18,
        flexShrink: 0,
      }}
    >
      <span>{current?.emoji}</span>
      <span
        style={{
          fontSize: 9,
          transform: open ? "rotate(180deg)" : "rotate(0deg)",
          display: "inline-block",
          transition: "transform 0.2s ease",
          color: "var(--color-fg-muted)",
        }}
      >
        <ChevronUp />
      </span>
    </button>
  );
}

export function EmojiPicker({
  isOpen,
  onClose,
  onSelect,
  customEmoji = [],
  skinTone,
  onSkinToneChange,
  anchorRef,
}: EmojiPickerProps) {
  const emojiData = useEmojiData();
  const [skinToneOpen, setSkinToneOpen] = useState(false);
  const [activeCategory, setActiveCategory] = useState(0);
  const viewportRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const nativeToId = useMemo(() => {
    if (!emojiData) return null;
    const map = new Map<string, string>();
    for (const [id, emoji] of Object.entries(emojiData.emojis)) {
      if (emoji.s[0]?.n) map.set(emoji.s[0].n, id);
    }
    return map;
  }, [emojiData]);

  // Position the picker relative to the anchor. We portal to
  // document.body and use position: fixed so the picker escapes the
  // chat sidebar's overflow-hidden wrapper (which would otherwise clip
  // the top of an 80vh-tall picker).
  useLayoutEffect(() => {
    if (!isOpen) return;
    const anchor = anchorRef.current;
    const container = containerRef.current;
    if (!anchor || !container) return;

    const PICKER_WIDTH = 352;
    const GAP = 8;
    const VIEWPORT_PAD = 8;

    const compute = () => {
      const anchorRect = anchor.getBoundingClientRect();
      const containerRect = container.getBoundingClientRect();
      const viewportH = window.innerHeight;

      // Default: above the anchor, right-aligned.
      let top = anchorRect.top - containerRect.height - GAP;
      let left = anchorRect.right - containerRect.width;

      // If there's not enough room above, flip below the anchor.
      if (top < VIEWPORT_PAD) {
        top = anchorRect.bottom + GAP;
      }

      // Clamp horizontally so we don't run off the left/right edges.
      const minLeft = VIEWPORT_PAD;
      const maxLeft = window.innerWidth - containerRect.width - VIEWPORT_PAD;
      left = Math.max(minLeft, Math.min(left, maxLeft));

      // If flipping below also overflows the viewport bottom, shrink
      // the available space by clamping top to fit.
      if (top + containerRect.height > viewportH - VIEWPORT_PAD) {
        top = Math.max(
          VIEWPORT_PAD,
          viewportH - containerRect.height - VIEWPORT_PAD,
        );
      }

      container.style.top = `${top}px`;
      container.style.left = `${left}px`;
      void PICKER_WIDTH; // referenced for documentation; layout uses measured width
    };

    // Initial position. The container starts with visibility: hidden so
    // the first paint doesn't flash at (0,0); we show it after measuring.
    container.style.visibility = "hidden";
    compute();
    container.style.visibility = "visible";

    window.addEventListener("resize", compute);
    window.addEventListener("scroll", compute, true);
    return () => {
      window.removeEventListener("resize", compute);
      window.removeEventListener("scroll", compute, true);
    };
  }, [isOpen, anchorRef]);

  // Escape closes.
  useEffect(() => {
    if (!isOpen) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen, onClose]);

  // Click-outside closes (unless the click is on the anchor; let the
  // anchor's own onClick toggle).
  useEffect(() => {
    if (!isOpen) return;
    const handlePointerDown = (e: PointerEvent) => {
      const target = e.target as Node | null;
      if (!target) return;
      if (containerRef.current?.contains(target)) return;
      if (anchorRef.current?.contains(target)) return;
      onClose();
    };
    document.addEventListener("pointerdown", handlePointerDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
    };
  }, [isOpen, onClose, anchorRef]);

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return;

    const handleScroll = () => {
      const sizer = viewport.querySelector<HTMLElement>(
        "[frimousse-list-sizer]",
      );
      // Skip index 0; it's the hidden measurement element frimousse renders
      const categories = Array.from(
        viewport.querySelectorAll<HTMLElement>("[frimousse-category]"),
      ).slice(1);
      const sizerOffset = sizer?.offsetTop ?? 0;
      const scrollTop = viewport.scrollTop;
      let active = 0;
      for (let i = 0; i < categories.length; i++) {
        if (sizerOffset + categories[i].offsetTop <= scrollTop + 8) active = i;
      }
      setActiveCategory(active);
    };

    viewport.addEventListener("scroll", handleScroll, { passive: true });
    return () => viewport.removeEventListener("scroll", handleScroll);
  }, [isOpen]);

  const scrollToCategory = useCallback((index: number) => {
    const viewport = viewportRef.current;
    if (!viewport) return;
    const sizer = viewport.querySelector<HTMLElement>("[frimousse-list-sizer]");
    // Skip index 0; it's the hidden measurement element frimousse renders
    const categories = Array.from(
      viewport.querySelectorAll<HTMLElement>("[frimousse-category]"),
    ).slice(1);
    const category = categories[index];
    const sizerOffset = sizer?.offsetTop ?? 0;
    if (category) {
      viewport.scrollTo({
        top: sizerOffset + category.offsetTop,
        behavior: "smooth",
      });
      setActiveCategory(index);
    }
  }, []);

  if (!isOpen) return null;
  const handleStandardSelect = (arg: { emoji: string } | string) => {
    const native = typeof arg === "string" ? arg : arg.emoji;
    onSelect?.({ type: "standard", native });
  };

  const handleCustomSelect = (name: string) => {
    onSelect?.({ type: "custom", name });
  };

  return createPortal(
    <div
      ref={containerRef}
      className="rounded-xl border border-(--color-border) bg-(--color-bg-elevated) shadow-2xl"
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        width: 352,
        zIndex: 1000,
        overflow: "hidden",
      }}
    >
      {customEmoji.length > 0 && (
        <div
          style={{
            borderBottomWidth: 1,
            borderBottomColor: "var(--color-border)",
            borderBottomStyle: "solid",
            padding: 8,
          }}
        >
          <span
            style={{
              display: "block",
              fontSize: 11,
              fontWeight: 600,
              color: "var(--color-fg-muted)",
              textTransform: "uppercase",
              letterSpacing: "0.05em",
              padding: "4px 4px 8px",
            }}
          >
            Custom
          </span>
          <div
            style={{
              display: "flex",
              flexWrap: "wrap",
              gap: 2,
            }}
          >
            {customEmoji.map((entry) => (
              <button
                key={entry.name}
                title={`:${entry.name}:`}
                onClick={() => handleCustomSelect(entry.name)}
                style={{
                  width: 36,
                  height: 36,
                  padding: 3,
                  borderRadius: 6,
                  border: "none",
                  background: "transparent",
                  cursor: "pointer",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                }}
                onMouseEnter={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background =
                    "var(--color-bg-overlay)";
                }}
                onMouseLeave={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background =
                    "transparent";
                }}
              >
                <img
                  src={entry.imageUrl}
                  alt={entry.alt ?? entry.name}
                  style={{
                    width: "100%",
                    height: "100%",
                    objectFit: "contain",
                  }}
                />
              </button>
            ))}
          </div>
        </div>
      )}
      <FrimousseEmojiPicker.Root
        onEmojiSelect={handleStandardSelect}
        style={{
          width: "100%",
          maxHeight: 420,
          height: "80vh",
          display: "flex",
          flexDirection: "column",
          background: "transparent",
        }}
      >
        <FrimousseEmojiPicker.Search
          style={{
            margin: "10px 10px 4px 10px",
            padding: "6px 10px",
            borderRadius: 8,
            border: "1px solid var(--color-border)",
            background: "var(--color-bg)",
            color: "var(--color-fg)",
            fontSize: 13,
            outline: "none",
            width: "calc(100% - 16px - 10px)",
            boxSizing: "border-box",
          }}
          placeholder="Search emoji…"
        />
        <div
          style={{
            display: "flex",
            justifyContent: "space-around",
            padding: "4px 14px",
            borderBottom: "1px solid var(--color-border)",
          }}
        >
          {CATEGORY_ICONS.map(({ label, Icon }, i) => (
            <button
              key={label}
              title={label}
              onClick={() => scrollToCategory(i)}
              style={{
                width: 30,
                height: 30,
                borderRadius: 6,
                border: "none",
                background:
                  activeCategory === i
                    ? "var(--color-bg-overlay)"
                    : "transparent",
                cursor: "pointer",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                color:
                  activeCategory === i
                    ? "var(--color-fg)"
                    : "var(--color-fg-muted)",
                transition: "color 0.15s ease, background 0.15s ease",
              }}
            >
              <Icon size={16} />
            </button>
          ))}
        </div>
        <FrimousseEmojiPicker.Viewport
          ref={viewportRef}
          style={{ flex: 1, position: "relative" }}
        >
          <FrimousseEmojiPicker.Loading
            style={{
              position: "absolute",
              inset: 0,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              color: "var(--color-fg-muted)",
              fontSize: 13,
            }}
          >
            Loading...
          </FrimousseEmojiPicker.Loading>
          <FrimousseEmojiPicker.Empty
            style={{
              position: "absolute",
              inset: 0,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              color: "var(--color-fg-muted)",
              fontSize: 13,
            }}
          >
            No emoji found.
          </FrimousseEmojiPicker.Empty>
          <FrimousseEmojiPicker.List
            style={{
              paddingBottom: 6,
              paddingLeft: 10,
              userSelect: "none",
            }}
            components={{
              CategoryHeader: ({ category, ...props }) => {
                const propsStyle = props.style || {};
                // override default styles!
                delete (props as { style?: object }).style;
                return (
                  <div
                    style={{
                      background: `linear-gradient(var(--color-bg-elevated), var(--color-bg-elevated), transparent)`,
                      height: "120%",
                      margin: "0 -10px 16px",
                      padding: "4px 16px",
                      ...propsStyle,
                    }}
                    {...(props as React.HTMLAttributes<HTMLDivElement>)}
                  >
                    {category.label}
                  </div>
                );
              },
              Row: ({ children, ...props }) => (
                <div
                  style={{ padding: "0 6px" }}
                  {...(props as React.HTMLAttributes<HTMLDivElement>)}
                >
                  {children}
                </div>
              ),
              Emoji: ({ emoji, ...props }) => {
                const propsStyle = props.style || {};
                // override default styles!
                delete (props as { style?: object }).style;
                return (
                  <button
                    style={{
                      width: 36,
                      height: 36,
                      fontSize: 22,
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      borderRadius: 6,
                      border: "none",
                      background: "transparent",
                      cursor: "pointer",
                      color: "inherit",
                      ...propsStyle,
                    }}
                    onMouseEnter={(e) => {
                      (e.currentTarget as HTMLButtonElement).style.background =
                        "var(--color-bg-overlay)";
                    }}
                    onMouseLeave={(e) => {
                      (e.currentTarget as HTMLButtonElement).style.background =
                        "transparent";
                    }}
                    {...(props as React.HTMLAttributes<HTMLButtonElement>)}
                  >
                    {emoji.emoji}
                  </button>
                );
              },
            }}
          />
        </FrimousseEmojiPicker.Viewport>
        <FrimousseEmojiPicker.ActiveEmoji>
          {({ emoji }) => {
            return (
              <div>
                <SkinToneTray
                  open={skinToneOpen}
                  setOpen={setSkinToneOpen}
                  skinTone={skinTone}
                  onSelect={(t) => onSkinToneChange?.(t)}
                />
                <div
                  style={{
                    padding: "6px 10px 6px 20px",
                    borderTop: "1px solid var(--color-border)",
                    display: "flex",
                    flexDirection: "row",
                    alignItems: "center",
                    gap: 8,
                    height: 46,
                  }}
                >
                  {emoji ? (
                    <>
                      <span style={{ fontSize: 22 }}>{emoji.emoji}</span>
                      <div
                        style={{
                          display: "flex",
                          flexDirection: "column",
                          flex: 1,
                          minWidth: 0,
                        }}
                      >
                        <span
                          style={{
                            fontSize: 13,
                            color: "var(--color-fg)",
                            whiteSpace: "nowrap",
                            overflow: "hidden",
                            textOverflow: "ellipsis",
                          }}
                        >
                          {emoji.label}
                        </span>
                        {nativeToId?.get(emoji.emoji) && (
                          <span
                            style={{
                              fontSize: 11,
                              color: "var(--color-fg-muted)",
                              fontFamily: "monospace",
                            }}
                          >
                            :{nativeToId.get(emoji.emoji)}:
                          </span>
                        )}
                      </div>
                    </>
                  ) : (
                    <span
                      style={{
                        color: "var(--color-fg-muted)",
                        fontSize: 13,
                        flex: 1,
                      }}
                    >
                      Select an emoji...
                    </span>
                  )}
                  <SkinToneTrigger
                    open={skinToneOpen}
                    setOpen={setSkinToneOpen}
                    skinTone={skinTone}
                  />
                </div>
              </div>
            );
          }}
        </FrimousseEmojiPicker.ActiveEmoji>
      </FrimousseEmojiPicker.Root>
    </div>,
    document.body,
  );
}
