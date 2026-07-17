import {
  Button,
  formatHandleWithAt,
  Text,
  useTheme,
  zero,
} from "@streamplace/components";
import { Palette, SwatchBook, X } from "lucide-react-native";
import { useEffect, useState } from "react";
import {
  Keyboard,
  Modal,
  Platform,
  Pressable,
  TouchableOpacity,
  View,
} from "react-native";
import ColorPicker, {
  HueSlider,
  InputWidget,
  Panel1,
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
  const { theme } = useTheme();
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

  const c = theme.colors;
  const modal = (
    <Modal
      visible={modalVisible}
      transparent={true}
      animationType="fade"
      onRequestClose={closeModal}
    >
      {/* Backdrop — tap outside to dismiss */}
      <Pressable
        onPress={closeModal}
        style={{
          flex: 1,
          alignItems: "center",
          justifyContent: "center",
          paddingHorizontal: 16,
          backgroundColor: c.overlay,
        }}
      >
        <Pressable
          onPress={(e) => e.stopPropagation()}
          style={{
            width: 400,
            maxWidth: "100%",
            maxHeight: "88%",
            padding: 20,
            borderRadius: theme.borderRadius.lg,
            backgroundColor: c.surface2,
            borderWidth: 1,
            borderColor: c.borderSubtle,
            ...theme.shadows.xl,
          }}
        >
          {/* Header */}
          <View
            style={{
              flexDirection: "row",
              alignItems: "center",
              justifyContent: "space-between",
              marginBottom: 16,
            }}
          >
            <View
              style={{ flexDirection: "row", alignItems: "center", gap: 10 }}
            >
              <Palette color={tempColor} size={18} />
              <Text style={{ color: c.text1, fontSize: 15, fontWeight: "600" }}>
                Name color
              </Text>
            </View>
            <TouchableOpacity
              onPress={closeModal}
              hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}
            >
              <X color={c.text3} size={18} />
            </TouchableOpacity>
          </View>

          {/* Preview — your handle in the chosen color */}
          {profile?.handle && (
            <View
              style={{
                alignItems: "center",
                paddingVertical: 16,
                marginBottom: 16,
                borderRadius: theme.borderRadius.md,
                backgroundColor: c.surface0,
                borderWidth: 1,
                borderColor: c.borderSubtle,
              }}
            >
              <Text
                size="xs"
                style={{
                  color: c.text3,
                  marginBottom: 6,
                  textTransform: "uppercase",
                  letterSpacing: 0.5,
                }}
              >
                Preview
              </Text>
              <Text size="xl" style={{ color: tempColor, fontWeight: "600" }}>
                {formatHandleWithAt(profile)}
              </Text>
            </View>
          )}

          {/* Color Picker */}
          <ColorPicker
            value={tempColor}
            onChangeJS={(result) => setTempColor(result.rgb)}
          >
            {/* Hex field with a swatch of the current pick */}
            <View style={{ flexDirection: "row", gap: 10, marginBottom: 14 }}>
              <View
                style={{
                  width: 46,
                  borderRadius: theme.borderRadius.md,
                  backgroundColor: tempColor,
                  borderWidth: 1,
                  borderColor: c.borderStrong,
                }}
              />
              <View style={{ flex: 1 }}>
                <InputWidget
                  defaultFormat="HEX"
                  formats={["HEX"]}
                  disableAlphaChannel
                  containerStyle={{
                    backgroundColor: c.surface0,
                    borderWidth: 1,
                    borderColor: c.borderStrong,
                    borderRadius: theme.borderRadius.md,
                    paddingHorizontal: 12,
                    paddingTop: 8,
                    paddingBottom: 4,
                  }}
                  inputStyle={{
                    color: c.text1,
                    borderColor: "transparent",
                  }}
                  inputTitleStyle={{ color: c.text3, fontSize: 10 }}
                />
              </View>
            </View>

            <Panel1
              style={{ borderRadius: theme.borderRadius.md, marginBottom: 14 }}
            />
            <HueSlider
              style={{
                borderRadius: theme.borderRadius.full,
                marginBottom: 16,
              }}
            />
            <Swatches style={{ marginBottom: 4 }} />
          </ColorPicker>

          {/* Actions */}
          <View style={{ flexDirection: "row", gap: 10, marginTop: 12 }}>
            <View style={{ flex: 1 }}>
              <Button variant="secondary" onPress={closeModal}>
                Cancel
              </Button>
            </View>
            <View style={{ flex: 1 }}>
              <Button variant="primary" onPress={saveColor}>
                Save color
              </Button>
            </View>
          </View>
        </Pressable>
      </Pressable>
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
  const { currentColor, openModal, modal } = useNameColorPicker();

  return (
    <View style={[zero.layout.flex.alignCenter, zero.layout.flex.row]}>
      <Button
        variant="secondary"
        leftIcon={<SwatchBook color={currentColor} />}
        style={[buttonProps?.style]}
        onPress={openModal}
        {...buttonProps}
      >
        <Text style={[{ color: currentColor, fontWeight: "600" }]}>
          {textProp ? textProp(currentColor) : "Change Name Color"}
        </Text>
      </Button>

      {modal}

      {children}
    </View>
  );
}
