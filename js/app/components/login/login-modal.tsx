import { Text, useTheme, zero } from "@streamplace/components";
import { X } from "lucide-react-native";
import {
  KeyboardAvoidingView,
  Modal,
  Platform,
  Pressable,
  TouchableOpacity,
  View,
} from "react-native";
import LoginForm from "./login-form";

interface LoginModalProps {
  visible: boolean;
  onClose: () => void;
  onOpenPdsModal: () => void;
}

export default function LoginModal({
  visible,
  onClose,
  onOpenPdsModal,
}: LoginModalProps) {
  const { theme } = useTheme();

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
          zero.layout.flex.center,
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
          style={[
            zero.r.xl,
            zero.p[6],
            {
              backgroundColor: theme.colors.surface2,
              borderWidth: 1,
              borderColor: theme.colors.borderStrong,
              width: 600,
              maxWidth: "95%",
              maxHeight: "85%",
              ...theme.shadows.xl,
            },
          ]}
          onPress={(e) => e.stopPropagation()}
        >
          <View
            style={[
              zero.layout.flex.row,
              zero.layout.flex.spaceBetween,
              zero.layout.flex.alignCenter,
              zero.mb[4],
            ]}
          >
            <Text size="4xl" leading="snug">
              Log in
            </Text>
            <TouchableOpacity
              onPress={onClose}
              hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}
            >
              <X color={theme.colors.text3} size={24} />
            </TouchableOpacity>
          </View>

          <LoginForm
            onSuccess={onClose}
            onCloseModal={onClose}
            onOpenPdsModal={onOpenPdsModal}
          />
        </Pressable>
      </KeyboardAvoidingView>
    </Modal>
  );
}
