import { hexToRgba, Text, useAQState, useTheme } from "@streamplace/components";
import {
  EmojiPicker as FrimousseEmojiPicker,
  SkinTone,
  useSkinTone,
} from "frimousse";
import {
  ChevronUp,
  Flag,
  Hash,
  Lightbulb,
  LucideIcon,
  PawPrint,
  PersonStanding,
  Pizza,
  Plane,
  Smile,
  Volleyball,
} from "lucide-react-native";
import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { View } from "react-native";
import { useEmojiData } from "utils/emoji";

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

export function SkinToneTray({ open, setOpen }: SkinTonePickerOpen) {
  const [aqSkinTone, setAQSkinTone] = useAQState(
    "emoji-picker-tone",
    "none" as SkinTone,
  );
  const [skinTone, setSkinTone, skinToneVariations] = useSkinTone("👋");

  useEffect(() => {
    setSkinTone(aqSkinTone);
  }, [aqSkinTone, setSkinTone]);

  const handleSelect = (tone: SkinTone) => {
    setAQSkinTone(tone);
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
        borderTop: open ? "1px solid rgba(255,255,255,0.07)" : "none",
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
                ? "1px solid rgba(255,255,255,0.35)"
                : "1px solid rgba(255,255,255,0.08)",
            background:
              skinTone === variation.skinTone
                ? "rgba(255,255,255,0.15)"
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

export function SkinToneTrigger({ open, setOpen }: SkinTonePickerOpen) {
  const [aqSkinTone] = useAQState("emoji-picker-tone", "none" as SkinTone);
  const [, , skinToneVariations] = useSkinTone("👋");
  const current =
    skinToneVariations.find((v) => v.skinTone === aqSkinTone) ??
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
        border: "1px solid rgba(255,255,255,0.08)",
        borderRadius: 6,
        background: open ? "rgba(255,255,255,0.1)" : "transparent",
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
          color: "grey",
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
}: EmojiPickerProps) {
  const { theme } = useTheme();
  const emojiData = useEmojiData();
  const [skinToneOpen, setSkinToneOpen] = useState(false);
  const [activeCategory, setActiveCategory] = useState(0);
  const viewportRef = useRef<HTMLDivElement>(null);

  const nativeToId = useMemo(() => {
    if (!emojiData) return null;
    const map = new Map<string, string>();
    for (const [id, emoji] of Object.entries(emojiData.emojis)) {
      if (emoji.s[0]?.n) map.set(emoji.s[0].n, id);
    }
    return map;
  }, [emojiData]);

  const containerRef = useRef<any>(null);

  useEffect(() => {
    if (!isOpen) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    const handlePointerDown = (e: PointerEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target)) {
        // ignore the trigger button itself as it'll close the picker anyways
        if (
          e.target instanceof HTMLElement &&
          e.target.closest("#web-emoji-picker-btn")
        ) {
          return;
        }
        onClose();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    document.addEventListener("pointerdown", handlePointerDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
      document.removeEventListener("pointerdown", handlePointerDown);
    };
  }, [isOpen, onClose]);

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return;

    const handleScroll = () => {
      const sizer = viewport.querySelector<HTMLElement>(
        "[frimousse-list-sizer]",
      );
      // Skip index 0 — it's the hidden measurement element frimousse renders
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
    // Skip index 0 — it's the hidden measurement element frimousse renders
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
  const handleStandardSelect = (arg: any) => {
    onSelect?.({ type: "standard", native: arg.emoji ?? arg });
  };

  const handleCustomSelect = (name: string) => {
    onSelect?.({ type: "custom", name });
  };

  return (
    <View
      ref={containerRef}
      style={{
        position: "absolute",
        bottom: 92,
        left: 0,
        width: 352,
        marginBottom: 8,
        zIndex: 1000,
        borderRadius: 12,
        overflow: "hidden",
        backgroundColor: theme.colors.background,
        boxShadow: "0 4px 24px rgba(0,0,0,0.4)",
      }}
    >
      {customEmoji.length > 0 && (
        <View
          style={{
            borderBottomWidth: 1,
            borderBottomColor: theme.colors.border,
            padding: 8,
          }}
        >
          <span
            style={{
              display: "block",
              fontSize: 11,
              fontWeight: 600,
              color: "rgba(255,255,255,0.4)",
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
                    "rgba(255,255,255,0.1)";
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
        </View>
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
            border: "1px solid rgba(255,255,255,0.1)",
            background: "rgba(255,255,255,0.07)",
            color: "white",
            fontSize: 13,
            outline: "none",
            width: "calc(100% - 16px - 10px)",
            boxSizing: "border-box",
          }}
          placeholder="Search emoji…"
          autoFocus
        />
        <div
          style={{
            display: "flex",
            justifyContent: "space-around",
            padding: "4px 14px",
            borderBottom: `1px solid ${theme.colors.border}`,
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
                    ? "rgba(255,255,255,0.12)"
                    : "transparent",
                cursor: "pointer",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                color:
                  activeCategory === i
                    ? "rgba(255,255,255,0.9)"
                    : "rgba(255,255,255,0.4)",
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
              color: "rgba(255,255,255,0.4)",
              fontSize: 13,
            }}
          >
            <Text>Loading...</Text>
          </FrimousseEmojiPicker.Loading>
          <FrimousseEmojiPicker.Empty
            style={{
              position: "absolute",
              inset: 0,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              color: "rgba(255,255,255,0.4)",
              fontSize: 13,
            }}
          >
            <Text>No emoji found.</Text>
          </FrimousseEmojiPicker.Empty>
          <FrimousseEmojiPicker.List
            style={{
              paddingBottom: 6,
              paddingLeft: 10,
              userSelect: "none",
            }}
            components={{
              CategoryHeader: ({ category, ...props }) => {
                let propsStyle = props.style || {};
                // override default styles!
                delete props.style;
                return (
                  <div
                    style={{
                      background: `linear-gradient(${theme.colors.background}, ${theme.colors.background}d0, transparent)`,
                      height: "120%",
                      margin: "0 -10px 16px",
                      padding: "4px 16px",
                      ...propsStyle,
                    }}
                    {...(props as any)}
                  >
                    <Text {...(props as any)}>{category.label}</Text>
                  </div>
                );
              },
              Row: ({ children, ...props }) => (
                <div style={{ padding: "0 6px" }} {...props}>
                  {children}
                </div>
              ),
              Emoji: ({ emoji, ...props }) => {
                let propsStyle = props.style || {};
                // override default styles!
                delete props.style;
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
                        hexToRgba(theme.colors.secondary, 0.56);
                    }}
                    onMouseLeave={(e) => {
                      (e.currentTarget as HTMLButtonElement).style.background =
                        "transparent";
                    }}
                    {...props}
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
                <SkinToneTray open={skinToneOpen} setOpen={setSkinToneOpen} />
                <div
                  style={{
                    padding: "6px 10px 6px 20px",
                    borderTop: `1px solid ${theme.colors.border}`,
                    display: "flex",
                    flexDirection: "row",
                    alignItems: "center",
                    gap: 8,
                    height: 46,
                  }}
                >
                  {emoji ? (
                    <>
                      <Text size="4xl">{emoji.emoji}</Text>
                      <div
                        style={{
                          display: "flex",
                          flexDirection: "column",
                          gap: -2,
                          flex: 1,
                          minWidth: 0,
                        }}
                      >
                        <Text size="sm">{emoji.label}</Text>
                        {nativeToId?.get(emoji.emoji) && (
                          <Text color="muted" size="xs">
                            :{nativeToId.get(emoji.emoji)}:
                          </Text>
                        )}
                      </div>
                    </>
                  ) : (
                    <Text color="muted" size="sm" style={{ flex: 1 }}>
                      Select an emoji...
                    </Text>
                  )}
                  <SkinToneTrigger
                    open={skinToneOpen}
                    setOpen={setSkinToneOpen}
                  />
                </div>
              </div>
            );
          }}
        </FrimousseEmojiPicker.ActiveEmoji>
      </FrimousseEmojiPicker.Root>
    </View>
  );
}
