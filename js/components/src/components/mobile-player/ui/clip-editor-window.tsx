import { X } from "lucide-react-native";
import { useMemo } from "react";
import { Pressable, ScrollView, useWindowDimensions, View } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
} from "react-native-reanimated";
import { Text, useTheme } from "../../ui";

const PANEL_WIDTH = 380;
const PANEL_MARGIN = 24;
const HEADER_HEIGHT = 44;

// Web-only: a draggable, non-modal overlay window for the clip editor. The
// player stays visible and interactive underneath — only the header drags the
// window. Native uses the fullscreen bottom sheet instead (see
// ClipEditorModal), so this component is never rendered there.
export function ClipEditorWindow({
  title,
  onClose,
  children,
}: {
  title: string;
  onClose: () => void;
  children: React.ReactNode;
}) {
  const th = useTheme();
  const { width: screenWidth, height: screenHeight } = useWindowDimensions();
  const width = Math.min(PANEL_WIDTH, screenWidth - PANEL_MARGIN);

  // Right-anchored (matching the player chrome): dragging right moves the
  // window right, decreasing the right offset.
  const top = useSharedValue(80);
  const right = useSharedValue(PANEL_MARGIN);
  const startTop = useSharedValue(0);
  const startRight = useSharedValue(0);

  const drag = useMemo(
    () =>
      Gesture.Pan()
        .onStart(() => {
          startTop.value = top.value;
          startRight.value = right.value;
        })
        .onUpdate((e) => {
          top.value = Math.max(0, startTop.value + e.translationY);
          right.value = Math.max(0, startRight.value - e.translationX);
        }),
    [top, right, startTop, startRight],
  );

  const panelStyle = useAnimatedStyle(() => ({
    top: top.value,
    right: right.value,
  }));

  return (
    <Animated.View
      style={[
        {
          // react-native-web supports fixed positioning; RN's types don't.
          position: "fixed" as any,
          width,
          zIndex: 9999,
          borderRadius: 12,
          backgroundColor: th.theme.colors.card,
          borderWidth: 1,
          borderColor: th.theme.colors.border,
          shadowColor: "#000",
          shadowOffset: { width: 0, height: 4 },
          shadowOpacity: 0.3,
          shadowRadius: 12,
          elevation: 8,
        },
        panelStyle,
      ]}
    >
      <GestureDetector gesture={drag}>
        <View
          style={{
            height: HEADER_HEIGHT,
            flexDirection: "row",
            alignItems: "center",
            justifyContent: "space-between",
            paddingHorizontal: 12,
            borderBottomWidth: 1,
            borderBottomColor: th.theme.colors.border,
          }}
        >
          <Text size="sm" weight="semibold">
            {title}
          </Text>
          <Pressable onPress={onClose} style={{ padding: 4 }}>
            <X size={18} color={th.theme.colors.foreground} />
          </Pressable>
        </View>
      </GestureDetector>
      <ScrollView
        style={{ maxHeight: Math.max(220, screenHeight - HEADER_HEIGHT - 140) }}
        contentContainerStyle={{ padding: 12 }}
        bounces={false}
      >
        {children}
      </ScrollView>
    </Animated.View>
  );
}
