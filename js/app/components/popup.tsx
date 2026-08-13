import { Text, zero } from "@streamplace/components";
import { Pressable, View, ViewStyle } from "react-native";

interface PopupProps {
  onClose: () => void;
  onPress?: () => void;
  containerProps: { style?: ViewStyle };
  bubbleProps: { style?: ViewStyle };
  children: React.ReactNode;
}

export default function Popup({
  onClose,
  containerProps,
  bubbleProps,
  children,
  onPress,
}: PopupProps) {
  return (
    <View
      style={[
        { position: "absolute" },
        zero.bottom[32],
        zero.flex.values[1],
        { alignItems: "center" },
        zero.w.percent[100],
        { backgroundColor: "blue", height: 0 },
        containerProps.style,
      ]}
    >
      <Pressable
        style={[
          zero.flex.values[1],
          { alignSelf: "stretch" },
          zero.p[4],
          zero.r.lg,
          { position: "absolute" },
          zero.bottom[0],
          {
            boxShadow: "0 0 10px 0 rgba(0, 0, 0, 0.1)", // token-ok: web shadow
            elevation: 5, // Android shadow
          },
          bubbleProps.style,
        ]}
        onPress={() => {
          if (onPress) {
            onPress();
          }
        }}
      >
        <Pressable
          style={[
            { position: "absolute" },
            zero.top[0],
            zero.right[0],
            { marginRight: -15, marginTop: -5 },
            zero.bg.transparent,
            zero.p[2],
          ]}
          onPress={(e) => {
            e.stopPropagation();
            onClose();
          }}
        >
          <Text style={[{ fontSize: 18 }]}>✕</Text>
        </Pressable>
        {children}
      </Pressable>
    </View>
  );
}
