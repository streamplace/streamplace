import { useTheme, useTranslation, zero } from "@streamplace/components";
import { X } from "lucide-react-native";
import {
  KeyboardAvoidingView,
  Modal,
  Platform,
  Pressable,
  ScrollView,
  TouchableOpacity,
  View,
} from "react-native";
import LoginForm from "./login-form";

interface LoginModalProps {
  visible: boolean;
  onClose: () => void;
}

export default function LoginModal({ visible, onClose }: LoginModalProps) {
  const { theme } = useTheme();
  const { t } = useTranslation("common");

  if (!visible) {
    return null;
  }

  return (
    <Modal
      visible={visible}
      transparent={true}
      animationType="fade"
      onRequestClose={onClose}
    >
      <KeyboardAvoidingView
        behavior={Platform.OS === "ios" ? "padding" : "height"}
        style={[
          zero.layout.flex[1],
          zero.layout.flex.alignCenter,
          zero.layout.flex.justifyCenter,
          {
            backgroundColor: theme.colors.overlay,
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
          style={{
            backgroundColor: theme.colors.surface1,
            borderWidth: 1,
            borderColor: theme.colors.border,
            borderRadius: theme.borderRadius.none,
            width: 400,
            maxWidth: "92%",
            maxHeight: "90%",
          }}
          onPress={(e) => e.stopPropagation()}
        >
          <ScrollView
            style={{ flexGrow: 0 }}
            contentContainerStyle={zero.p[6]}
            keyboardShouldPersistTaps="handled"
          >
            <View
              style={[
                zero.layout.flex.row,
                zero.layout.flex.justify.end,
                zero.mb[2],
              ]}
            >
              <TouchableOpacity
                testID="login-close"
                accessibilityLabel={t("close")}
                onPress={onClose}
                hitSlop={theme.spacing[3]}
              >
                <X color={theme.colors.text3} size={20} />
              </TouchableOpacity>
            </View>
            <LoginForm onSuccess={onClose} />
          </ScrollView>
        </Pressable>
      </KeyboardAvoidingView>
    </Modal>
  );
}
