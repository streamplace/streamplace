import { Button, formatHandleWithAt, zero } from "@streamplace/components";
import { Palette, SwatchBook, X } from "lucide-react-native";
import { useEffect, useState } from "react";
import {
  Keyboard,
  Modal,
  Platform,
  Pressable,
  Text,
  TouchableOpacity,
  View,
} from "react-native";
import ColorPicker, {
  HueSlider,
  Panel1,
  Preview,
  Swatches,
} from "reanimated-color-picker";
import { useStore } from "store";
import { useChatProfile, useUserProfile } from "store/hooks";
import { PlaceStreamChatProfile } from "streamplace";

/**
 * Parses an RGB color string and returns an object with red, green, and blue values
 */
function parseRgbString(rgbString: string): PlaceStreamChatProfile.Color {
  if (
    !rgbString ||
    (!rgbString.startsWith("rgb(") && !rgbString.startsWith("rgba("))
  ) {
    throw new Error("Invalid color string (not rgb or rgba)");
  }

  const numbersString = rgbString.replace(/^rgba?\(|\)$/g, "");
  const parts = numbersString.split(",");

  if (parts.length < 3) {
    throw new Error("Invalid color string (not enough parts)");
  }

  return {
    red: parseInt(parts[0].trim(), 10),
    green: parseInt(parts[1].trim(), 10),
    blue: parseInt(parts[2].trim(), 10),
  };
}

export function useNameColorPicker() {
  const [modalVisible, setModalVisible] = useState(false);
  const [tempColor, setTempColor] = useState("#bd6e86");
  const createChatProfileRecord = useStore(
    (state) => state.createChatProfileRecord,
  );
  const getChatProfileRecordFromPDS = useStore(
    (state) => state.getChatProfileRecordFromPDS,
  );
  const chatProfile = useChatProfile();
  const profile = useUserProfile();
  const isWeb = Platform.OS === "web";

  const currentColor = chatProfile?.profile?.color
    ? `rgb(${chatProfile.profile.color.red}, ${chatProfile.profile.color.green}, ${chatProfile.profile.color.blue})`
    : "#bd6e86";

  useEffect(() => {
    if (profile?.did && !chatProfile?.profile) {
      getChatProfileRecordFromPDS();
    }
    setTempColor(currentColor);
  }, [profile?.did, chatProfile?.profile?.color, currentColor]);

  const openModal = () => {
    if (!isWeb) {
      Keyboard.dismiss();
    }
    setTempColor(currentColor);
    setModalVisible(true);
  };

  const closeModal = () => {
    setModalVisible(false);
    setTempColor(currentColor);
  };

  const saveColor = () => {
    setModalVisible(false);
    const parsed = parseRgbString(tempColor);
    createChatProfileRecord(parsed.red, parsed.green, parsed.blue);
  };

  const modal = (
    <Modal
      visible={modalVisible}
      transparent={true}
      animationType="fade"
      onRequestClose={closeModal}
    >
      <View
        style={[
          zero.layout.flex[1],
          zero.layout.flex.center,
          zero.layout.flex.alignCenter,
          zero.layout.flex.justifyCenter,
          {
            backgroundColor: "rgba(0, 0, 0, 0.6)",
            position: "absolute",
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            width: "100%",
            height: "100%",
          },
        ]}
      >
        <Pressable
          style={[
            zero.bg.gray[900],
            zero.r.xl,
            zero.p[6],
            { width: 420, maxWidth: "90%", maxHeight: "85%" },
          ]}
          onPress={(e) => e.stopPropagation()}
        >
          {/* Header */}
          <View
            style={[
              zero.layout.flex.row,
              zero.layout.flex.spaceBetween,
              zero.layout.flex.alignCenter,
              zero.mb[5],
            ]}
          >
            <View
              style={[
                zero.layout.flex.row,
                zero.layout.flex.alignCenter,
                zero.gap.all[3],
              ]}
            >
              <Palette color={tempColor} size={20} />
              <Text style={[{ color: tempColor, fontWeight: "bold" }]}>
                Choose Color
              </Text>
            </View>
            <TouchableOpacity
              style={[zero.p[1]]}
              onPress={closeModal}
              hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}
            >
              <X color="#888" size={20} />
            </TouchableOpacity>
          </View>

          {/* User Preview */}
          {profile?.handle && (
            <View
              style={[
                zero.bg.gray[800],
                zero.r.md,
                zero.p[3],
                zero.mb[5],
                zero.layout.flex.alignCenter,
              ]}
            >
              <Text style={[{ color: tempColor, fontWeight: "600" }]}>
                @{profile.handle}
              </Text>
              <Text
                style={[
                  zero.text.gray[400],
                  { textTransform: "uppercase", letterSpacing: 1 },
                ]}
              >
                Preview
              </Text>
            </View>
          )}

          {/* Color Picker */}
          <View style={[zero.mb[5]]}>
            <ColorPicker
              value={tempColor}
              onCompleteJS={(result) => setTempColor(result.rgb)}
            >
              <View style={[zero.mb[3]]}>
                <Preview style={[zero.r.md]} />
              </View>
              <View style={[zero.mb[3]]}>
                <Panel1 style={[zero.r.md]} />
              </View>
              <View style={[zero.mb[3]]}>
                <HueSlider style={[zero.r.sm]} />
              </View>
              <View style={[zero.mb[3]]}>
                <Swatches style={[zero.r.sm]} />
              </View>
            </ColorPicker>
          </View>

          {/* Actions */}
          <View style={[zero.layout.flex.row, zero.gap.all[3]]}>
            <TouchableOpacity
              style={[
                zero.layout.flex[1],
                zero.bg.gray[700],
                zero.r.md,
                zero.p[3],
                zero.layout.flex.center,
              ]}
              onPress={closeModal}
            >
              <Text style={[zero.text.white, { fontWeight: "600" }]}>
                Cancel
              </Text>
            </TouchableOpacity>
            <TouchableOpacity
              style={[
                zero.layout.flex[1],
                zero.r.md,
                zero.p[3],
                zero.layout.flex.center,
                { backgroundColor: tempColor },
              ]}
              onPress={saveColor}
            >
              <Text style={[zero.text.white, { fontWeight: "600" }]}>
                Save Color
              </Text>
            </TouchableOpacity>
          </View>
        </Pressable>
      </View>
    </Modal>
  );

  return {
    currentColor,
    openModal,
    modal,
  };
}

export default function NameColorPicker({
  children,
  text: textProp,
  buttonProps,
}: {
  children?: React.ReactNode;
  text?: (color: string) => React.ReactNode;
  buttonProps?: any;
}) {
  const [modalVisible, setModalVisible] = useState(false);
  const [tempColor, setTempColor] = useState("#bd6e86");
  const createChatProfileRecord = useStore(
    (state) => state.createChatProfileRecord,
  );
  const getChatProfileRecordFromPDS = useStore(
    (state) => state.getChatProfileRecordFromPDS,
  );
  const chatProfile = useChatProfile();
  const profile = useUserProfile();
  const isWeb = Platform.OS === "web";

  const currentColor = chatProfile?.profile?.color
    ? `rgb(${chatProfile.profile.color.red}, ${chatProfile.profile.color.green}, ${chatProfile.profile.color.blue})`
    : "#bd6e86";

  useEffect(() => {
    if (profile?.did && !chatProfile?.profile) {
      getChatProfileRecordFromPDS();
    }
    setTempColor(currentColor);
  }, [profile?.did, chatProfile?.profile?.color, currentColor]);

  const handleOpenModal = () => {
    if (!isWeb) {
      Keyboard.dismiss();
    }
    setTempColor(currentColor);
    setModalVisible(true);
  };

  const handleCloseModal = () => {
    setModalVisible(false);
    setTempColor(currentColor); // Reset to current color on cancel
  };

  const handleSaveColor = () => {
    setModalVisible(false);
    const parsed = parseRgbString(tempColor);
    createChatProfileRecord(parsed.red, parsed.green, parsed.blue);
  };

  return (
    <View style={[zero.layout.flex.alignCenter, zero.layout.flex.row]}>
      <Button
        variant="secondary"
        leftIcon={<SwatchBook color={currentColor} />}
        style={[buttonProps?.style]}
        onPress={handleOpenModal}
        {...buttonProps}
      >
        <Text style={[{ color: currentColor, fontWeight: "600" }]}>
          {textProp ? textProp(currentColor) : "Change Name Color"}
        </Text>
      </Button>

      <Modal
        visible={modalVisible}
        transparent={true}
        animationType="fade"
        onRequestClose={handleCloseModal}
      >
        <View
          style={[
            zero.layout.flex[1],
            zero.layout.flex.center,
            zero.layout.flex.alignCenter,
            zero.layout.flex.justifyCenter,
            {
              backgroundColor: "rgba(0, 0, 0, 0.6)",
              position: "absolute",
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              width: "100%",
              height: "100%",
            },
          ]}
        >
          <Pressable
            style={[
              zero.bg.gray[900],
              zero.r.xl,
              zero.p[6],
              { width: 420, maxWidth: "90%", maxHeight: "85%" },
            ]}
            onPress={(e) => e.stopPropagation()}
          >
            {/* Header */}
            <View
              style={[
                zero.layout.flex.row,
                zero.layout.flex.spaceBetween,
                zero.layout.flex.alignCenter,
                zero.mb[5],
              ]}
            >
              <View
                style={[
                  zero.layout.flex.row,
                  zero.layout.flex.alignCenter,
                  zero.gap.all[3],
                ]}
              >
                <Palette color={tempColor} size={20} />
                <Text style={[{ color: tempColor, fontWeight: "bold" }]}>
                  Choose Color
                </Text>
              </View>
              <TouchableOpacity
                style={[zero.p[1]]}
                onPress={handleCloseModal}
                hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}
              >
                <X color="#888" size={20} />
              </TouchableOpacity>
            </View>

            {/* User Preview */}
            {profile?.handle && (
              <View
                style={[
                  zero.bg.gray[800],
                  zero.r.md,
                  zero.p[3],
                  zero.mb[5],
                  zero.layout.flex.alignCenter,
                ]}
              >
                <Text style={[{ color: tempColor, fontWeight: "600" }]}>
                  {formatHandleWithAt(profile)}
                </Text>
                <Text
                  style={[
                    zero.text.gray[400],
                    { textTransform: "uppercase", letterSpacing: 1 },
                  ]}
                >
                  Preview
                </Text>
              </View>
            )}

            {/* Color Picker */}
            <View style={[zero.mb[5]]}>
              <ColorPicker
                value={tempColor}
                onCompleteJS={(result) => setTempColor(result.rgb)}
              >
                <View style={[zero.mb[3]]}>
                  <Preview style={[zero.r.md]} />
                </View>
                <View style={[zero.mb[3]]}>
                  <Panel1 style={[zero.r.md]} />
                </View>
                <View style={[zero.mb[3]]}>
                  <HueSlider style={[zero.r.sm]} />
                </View>
                <View style={[zero.mb[3]]}>
                  <Swatches style={[zero.r.sm]} />
                </View>
              </ColorPicker>
            </View>

            {/* Actions */}
            <View style={[zero.layout.flex.row, zero.gap.all[3]]}>
              <TouchableOpacity
                style={[
                  zero.layout.flex[1],
                  zero.bg.gray[700],
                  zero.r.md,
                  zero.p[3],
                  zero.layout.flex.center,
                ]}
                onPress={handleCloseModal}
              >
                <Text style={[zero.text.white, { fontWeight: "600" }]}>
                  Cancel
                </Text>
              </TouchableOpacity>
              <TouchableOpacity
                style={[
                  zero.layout.flex[1],
                  zero.r.md,
                  zero.p[3],
                  zero.layout.flex.center,
                  { backgroundColor: tempColor },
                ]}
                onPress={handleSaveColor}
              >
                <Text style={[zero.text.white, { fontWeight: "600" }]}>
                  Save Color
                </Text>
              </TouchableOpacity>
            </View>
          </Pressable>
        </View>
      </Modal>

      {children}
    </View>
  );
}
