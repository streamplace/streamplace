import { X } from "@tamagui/lucide-icons";
import { Button, View, ViewProps } from "tamagui";

export default function Popup({
  onClose,
  containerProps: viewProps,
  bubbleProps: bubbleProps,
  children,
  onPress,
}: {
  onClose: () => void;
  onPress?: () => void;
  containerProps: ViewProps;
  bubbleProps: ViewProps;
  children: React.ReactNode;
}) {
  return (
    <View
      position="absolute"
      bottom="$8"
      f={1}
      alignItems="center"
      width="100%"
      {...viewProps}
    >
      <View
        f={1}
        alignItems="stretch"
        padding="$4"
        borderRadius="$4"
        onPress={() => {
          if (onPress) {
            onPress();
          }
        }}
        position="relative"
        boxShadow="0 0 10px 0 rgba(0, 0, 0, 0.1)"
        {...bubbleProps}
      >
        <Button
          position="absolute"
          top="$0"
          right="$0"
          onPress={(e) => {
            e.stopPropagation();
            onClose();
          }}
          marginRight={-15}
          marginTop={-5}
          backgroundColor="transparent"
        >
          <X />
        </Button>
        {children}
      </View>
    </View>
  );
}
