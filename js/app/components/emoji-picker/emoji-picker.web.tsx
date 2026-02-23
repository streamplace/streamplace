import { EmojiPicker as FrimousseEmojiPicker } from "frimousse";
import { View } from "react-native";
import { emojiEmitter } from "./emoji-emitter";

export type SelectedEmoji =
  | { type: "standard"; native: string }
  | { type: "custom"; name: string };

interface EmojiPickerProps {
  isOpen: boolean;
  onClose: () => void;
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
  customEmoji = [],
}: EmojiPickerProps) {
  if (!isOpen) return null;

  const handleStandardSelect = (emoji: { emoji: string }) => {
    emojiEmitter.emit("emoji-selected", {
      type: "standard",
      native: emoji.emoji,
    } satisfies SelectedEmoji);
    onClose();
  };

  const handleCustomSelect = (name: string) => {
    emojiEmitter.emit("emoji-selected", {
      type: "custom",
      name,
    } satisfies SelectedEmoji);
    onClose();
  };

  return (
    <View
      style={{
        position: "absolute",
        bottom: "100%",
        left: -115,
        width: 352,
        marginBottom: 8,
        zIndex: 1000,
        borderRadius: 12,
        overflow: "hidden",
        backgroundColor: "#1f2937",
        boxShadow: "0 4px 24px rgba(0,0,0,0.4)",
      }}
    >
      {customEmoji.length > 0 && (
        <View
          style={{
            borderBottomWidth: 1,
            borderBottomColor: "rgba(255,255,255,0.1)",
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
          height: 320,
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
            Loading…
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
            No emoji found.
          </FrimousseEmojiPicker.Empty>
          <FrimousseEmojiPicker.List
            style={{ paddingBottom: 6, userSelect: "none" }}
            components={{
              CategoryHeader: ({ category, ...props }) => (
                <div
                  style={{
                    padding: "8px 12px 4px",
                    fontSize: 11,
                    fontWeight: 600,
                    color: "rgba(255,255,255,0.4)",
                    textTransform: "uppercase",
                    letterSpacing: "0.05em",
                    background: "#1f2937",
                  }}
                  {...props}
                >
                  {category.label}
                </div>
              ),
              Row: ({ children, ...props }) => (
                <div style={{ padding: "0 6px" }} {...props}>
                  {children}
                </div>
              ),
              Emoji: ({ emoji, ...props }) => (
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
                  }}
                  onMouseEnter={(e) => {
                    (e.currentTarget as HTMLButtonElement).style.background =
                      "rgba(255,255,255,0.1)";
                  }}
                  onMouseLeave={(e) => {
                    (e.currentTarget as HTMLButtonElement).style.background =
                      "transparent";
                  }}
                  {...props}
                >
                  {emoji.emoji}
                </button>
              ),
            }}
          />
        </FrimousseEmojiPicker.Viewport>
      </FrimousseEmojiPicker.Root>
    </View>
  );
}
