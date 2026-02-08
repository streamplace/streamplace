import Picker from "@emoji-mart/react";
import { Platform, TouchableWithoutFeedback, View } from "react-native";
import { emojiEmitter } from "./emoji-emitter";

export type Emoji = {
  native: string;
};

interface EmojiPickerProps {
  isOpen: boolean;
  onClose: () => void;
}

export function EmojiPicker({ isOpen, onClose }: EmojiPickerProps) {
  if (!isOpen) {
    return null;
  }

  const isWeb = Platform.OS === "web";

  const onEmojiSelect = (emoji: Emoji) => {
    emojiEmitter.emit("emoji-selected", emoji);
    onClose();
  };

  return (
    <View
      style={[
        {
          position: "absolute",
          bottom: isWeb ? "100%" : undefined,
          left: -115,
          width: 380,
          marginBottom: 8,
          zIndex: 1000,
        },
      ]}
    >
      <TouchableWithoutFeedback onPress={onClose}>
        <View
          style={[
            {
              position: "absolute",
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
            },
          ]}
        />
      </TouchableWithoutFeedback>
      <View style={[{ position: "relative", width: 380 }]}>
        <Picker
          data={async () => {
            let data;
            if (isWeb) {
              data = (await import("../../assets/emoji-data.json")).default;
            } else {
              data = require("../../assets/emoji-data.json");
            }
            return data;
          }}
          onEmojiSelect={onEmojiSelect}
          autoFocus={true}
        />
      </View>
    </View>
  );
}
