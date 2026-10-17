import { Keyboard, Pressable } from "react-native";
import { useKeyboardSlide } from "../../../hooks";
import * as atoms from "../../../lib/theme/atoms";
import {
  borderAlphas,
  borderRadius,
  fontFamilies,
  scrims,
  spacing,
  statusColors,
  typeScale,
} from "../../../lib/theme/tokens";
import { Input, Text, useTheme, View } from "../../ui";
const { gap, h, layout, mt, position, sizes, w } = atoms;

// Over-video hairline, matched to the connection HUD.
const HUD_BORDER = borderAlphas.dark.strong;

type InputPanelProps = {
  title: string | undefined;
  setTitle: (title: string) => void;
  toggleGoLive: () => void;
  isLive: boolean;
  toggleStopStream?: () => void;
};

export function InputPanel({
  title,
  setTitle,
  toggleGoLive,
  isLive,
  toggleStopStream,
}: InputPanelProps) {
  const { slideKeyboard } = useKeyboardSlide();
  const { theme } = useTheme();
  const c = theme.colors;

  return (
    <View
      style={[
        layout.position.absolute,
        h.percent[30],
        position.bottom[0],
        w.percent[100],
        layout.flex.center,
        { transform: [{ translateY: slideKeyboard }] },
      ]}
    >
      <View
        style={[
          layout.flex.column,
          gap.all[3],
          sizes.maxWidth[80],
          { width: "100%", maxWidth: 420, padding: 10 },
        ]}
      >
        {!isLive && (
          <View
            style={{
              backgroundColor: scrims.dark,
              borderRadius: borderRadius.lg,
              borderWidth: 1,
              borderColor: HUD_BORDER,
            }}
          >
            <Input
              value={title}
              onChange={setTitle}
              placeholder="Enter stream title"
              onEndEditing={Keyboard.dismiss}
            />
          </View>
        )}
        {isLive ? (
          <Pressable
            onPress={toggleStopStream}
            style={{
              flexDirection: "row",
              alignItems: "center",
              justifyContent: "center",
              gap: spacing[2],
              paddingVertical: spacing[3],
              paddingHorizontal: spacing[4],
              backgroundColor: c.danger,
              borderRadius: borderRadius.lg,
            }}
          >
            <View
              style={{
                width: 10,
                height: 10,
                borderRadius: 2,
                backgroundColor: c.liveForeground,
              }}
            />
            <Text
              style={{
                color: c.liveForeground,
                fontFamily: fontFamilies.semiBold,
                fontWeight: "600",
                fontSize: typeScale.md.fontSize,
              }}
            >
              Stop stream
            </Text>
          </Pressable>
        ) : (
          <View style={[layout.flex.center, gap.all[2]]}>
            {/* Primary CTA: solid, high-contrast, with a live-red record dot —
                the record button grammar broadcasters expect. */}
            <Pressable
              onPress={toggleGoLive}
              style={{
                width: "100%",
                flexDirection: "row",
                alignItems: "center",
                justifyContent: "center",
                gap: spacing[2],
                paddingVertical: spacing[3],
                paddingHorizontal: spacing[4],
                backgroundColor: c.text,
                borderRadius: borderRadius.lg,
              }}
            >
              <View
                style={{
                  width: 9,
                  height: 9,
                  borderRadius: borderRadius.full,
                  backgroundColor: statusColors.live,
                }}
              />
              <Text
                style={{
                  color: c.surface0,
                  fontFamily: fontFamilies.semiBold,
                  fontWeight: "600",
                  fontSize: typeScale.md.fontSize,
                }}
              >
                Go live
              </Text>
            </Pressable>
            <Text color="muted" size="xs" style={[mt[1]]}>
              We'll announce that you're live on Bluesky.
            </Text>
          </View>
        )}
      </View>
    </View>
  );
}
