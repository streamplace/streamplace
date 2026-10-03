import {
  CAPTION_WEB_FONTS,
  CaptionDisplayPrefs,
  CaptionFont,
  captionColor,
  captionEdgeShadows,
  captionFontSize,
  captionTextShadowCss,
} from "@streamplace/core";
import { useState } from "react";
import {
  LayoutChangeEvent,
  Platform,
  Text as RNText,
  StyleSheet,
  TextStyle,
  View,
} from "react-native";
import { useTheme } from "../../lib/theme/theme";
import { typeScale } from "../../lib/theme/tokens";
import { usePlayerStore } from "../../player-store";
import { useCaptionPrefs } from "../../streamplace-store";
import { useCaptionLines, useLoadCaptionTracks } from "./use-captions";

// Platform font names for each CEA-708 caption font style.
const NATIVE_FONTS: Record<CaptionFont, string | undefined> =
  Platform.OS === "ios"
    ? {
        proportionalSans: undefined,
        monospacedSans: "Menlo",
        proportionalSerif: "Georgia",
        monospacedSerif: "Courier",
        casual: "Chalkboard SE",
        cursive: "Snell Roundhand",
        smallCaps: undefined,
      }
    : {
        proportionalSans: "sans-serif",
        monospacedSans: "monospace",
        proportionalSerif: "serif",
        monospacedSerif: "serif-monospace",
        casual: "casual",
        cursive: "cursive",
        smallCaps: "sans-serif-smallcaps",
      };

/**
 * Text style for caption lines under the viewer's display settings. Every
 * color here is the viewer's own choice (see @streamplace/core prefs),
 * not a design token.
 */
export function captionTextStyle(
  prefs: CaptionDisplayPrefs,
  fontSize: number,
): TextStyle {
  const shadows = captionEdgeShadows(prefs, fontSize);
  const shadow = shadows[0];
  const style: TextStyle = {
    fontSize,
    lineHeight: Math.round(fontSize * 1.3),
    color: captionColor(prefs.textColor, prefs.textOpacity),
    backgroundColor: captionColor(
      prefs.backgroundColor,
      prefs.backgroundOpacity,
    ),
    fontFamily:
      Platform.OS === "web"
        ? CAPTION_WEB_FONTS[prefs.font]
        : NATIVE_FONTS[prefs.font],
    fontVariant: prefs.font === "smallCaps" ? ["small-caps"] : undefined,
  };
  if (Platform.OS === "web" && shadows.length > 0) {
    // react-native-web passes textShadow through as CSS, which can stack
    // the several shadows an outline needs.
    return { ...style, textShadow: captionTextShadowCss(shadows) } as TextStyle;
  }
  if (shadow) {
    return {
      ...style,
      textShadowColor: shadow.color,
      textShadowOffset: { width: shadow.offsetX, height: shadow.offsetY },
      textShadowRadius: shadow.blur,
    };
  }
  return style;
}

/** Caption lines drawn in the viewer's style; also used as a preview. */
export function CaptionLines({
  lines,
  prefs,
  fontSize,
}: {
  lines: string[];
  prefs: CaptionDisplayPrefs;
  fontSize: number;
}) {
  const { theme } = useTheme();
  const textStyle = captionTextStyle(prefs, fontSize);
  const { backgroundColor, ...lineStyle } = textStyle;
  return (
    <View
      style={{
        maxWidth: "90%",
        alignItems: "center",
        paddingHorizontal: theme.spacing[2],
        paddingVertical: theme.spacing[1],
        borderRadius: theme.borderRadius.sm,
        backgroundColor: captionColor(prefs.windowColor, prefs.windowOpacity),
      }}
    >
      <RNText
        testID="caption-overlay-text"
        style={[lineStyle, { textAlign: "center" }]}
      >
        {lines.map((line, i) => (
          <RNText key={i} style={{ backgroundColor }}>
            {i > 0 ? "\n" : ""}
            {`\u00a0${line}\u00a0`}
          </RNText>
        ))}
      </RNText>
    </View>
  );
}

/**
 * Draws the selected caption track over the player, in the viewer's
 * caption style. Mount one per player, above the video; it also loads
 * the server's track list for the player's stream or video.
 */
export function CaptionOverlay() {
  useLoadCaptionTracks();
  const lines = useCaptionLines();
  const prefs = useCaptionPrefs();
  const showControls = usePlayerStore((x) => x.showControls);
  const { theme } = useTheme();
  const [height, setHeight] = useState(0);
  const onLayout = (e: LayoutChangeEvent) =>
    setHeight(e.nativeEvent.layout.height);
  const fontSize = captionFontSize(prefs, height, typeScale.xs.fontSize);

  return (
    <View
      testID="caption-overlay"
      pointerEvents="none"
      onLayout={onLayout}
      style={[
        StyleSheet.absoluteFill,
        {
          justifyContent: "flex-end",
          alignItems: "center",
          paddingHorizontal: theme.spacing[4],
          paddingBottom: theme.spacing[6],
          // Clear the bottom controls while they show. A transform rather
          // than padding: a layout change makes the browser hit-test a
          // resting pointer again, and once the faded controls let it
          // through, the player's hover would bring them straight back.
          transform: [
            {
              translateY: showControls
                ? theme.spacing[6] - theme.spacing[16]
                : 0,
            },
          ],
        },
      ]}
    >
      {lines.length > 0 && (
        <CaptionLines lines={lines} prefs={prefs} fontSize={fontSize} />
      )}
    </View>
  );
}
