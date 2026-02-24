import { Text, useTheme } from "@streamplace/components";
import { EmojiPicker as FrimousseEmojiPicker } from "frimousse";
import { useEffect, useMemo, useRef } from "react";
import { View } from "react-native";
import { useEmojiData } from "utils/emoji";

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

export function EmojiPicker({
  isOpen,
  onClose,
  onSelect,
  customEmoji = [],
}: EmojiPickerProps) {
  const { theme } = useTheme();
  const emojiData = useEmojiData();

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
      console.log("got click");
      if (containerRef.current && !containerRef.current.contains(e.target)) {
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

  if (!isOpen) return null;

  console.log(
    "[EmojiPicker.web] rendering, onSelect:",
    !!onSelect,
    "customEmoji:",
    customEmoji.length,
  );

  const handleStandardSelect = (arg: any) => {
    console.log(
      "[EmojiPicker.web] handleStandardSelect raw arg:",
      arg,
      "onSelect:",
      !!onSelect,
    );
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
          display: "flex",
          flexDirection: "column",
          background: "transparent",
        }}
      >
        <FrimousseEmojiPicker.Search
          style={{
            margin: "8px 8px 4px",
            padding: "6px 10px",
            borderRadius: 8,
            border: "1px solid rgba(255,255,255,0.1)",
            background: "rgba(255,255,255,0.07)",
            color: "white",
            fontSize: 13,
            outline: "none",
            width: "calc(100% - 16px)",
            boxSizing: "border-box",
          }}
          placeholder="Search emoji…"
          autoFocus
        />
        <FrimousseEmojiPicker.Viewport
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
                        theme.colors.secondary + "90";
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
              <div
                style={{
                  padding: "6px 20px",
                  borderTop: `1px solid ${theme.colors.border}`,
                }}
              >
                {emoji ? (
                  <div
                    style={{
                      display: "flex",
                      flexDirection: "row",
                      alignItems: "center",
                      gap: 8,
                      height: 46,
                    }}
                  >
                    <Text size="4xl">{emoji.emoji}</Text>
                    <div
                      style={{
                        display: "flex",
                        flexDirection: "column",
                        gap: -2,
                      }}
                    >
                      <Text size="sm">{emoji.label}</Text>
                      {nativeToId?.get(emoji.emoji) && (
                        <Text color="muted" size="xs">
                          :{nativeToId.get(emoji.emoji)}:
                        </Text>
                      )}
                    </div>
                  </div>
                ) : (
                  <div
                    style={{
                      display: "flex",
                      flexDirection: "row",
                      alignItems: "center",
                      gap: 8,
                      height: 46,
                    }}
                  >
                    <Text color="muted" size="sm">
                      Select an emoji...
                    </Text>
                  </div>
                )}
              </div>
            );
          }}
        </FrimousseEmojiPicker.ActiveEmoji>
      </FrimousseEmojiPicker.Root>
    </View>
  );
}
