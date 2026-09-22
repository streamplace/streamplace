import { Avatar, LiveBadge, Text, useTheme } from "@streamplace/components";
import { relativeAge } from "@streamplace/components/src/components/chat/chat-message";
import { scrims } from "@streamplace/components/src/lib/theme/tokens";
import { Image } from "expo-image";
import { Eye, Play } from "lucide-react-native";
import { ReactNode } from "react";
import { View } from "react-native";
import AQLink from "../aqlink";

// A stream or a video as a post in the social shell's landing feed (the
// client design's post card): avatar, display name, @handle and age on one
// line, the title as the post text, a 16:9 picture with a play mark, and a
// stats row. The whole card links to the page.

function formatCount(n: number): string {
  if (n >= 1_000_000)
    return `${(n / 1_000_000).toFixed(n < 10_000_000 ? 1 : 0)}M`;
  if (n >= 10_000) return `${Math.round(n / 1000)}K`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`;
  return `${n}`;
}

export function LandingPostCard({
  to,
  avatarUrl,
  name,
  handle,
  createdAt,
  text,
  imageUri,
  imageFallback,
  live = false,
  count,
  countLabel,
  corner,
}: {
  to: any;
  avatarUrl?: string;
  name: string;
  handle: string;
  createdAt?: string;
  text: string;
  imageUri?: string;
  /** Shown when there is no image (a video without a thumbnail). */
  imageFallback?: any;
  live?: boolean;
  /** The count in the stats row: viewers for a stream, views for a video. */
  count?: number;
  countLabel?: string;
  /** Something for the picture's bottom-right corner (a duration chip). */
  corner?: ReactNode;
}) {
  const { theme } = useTheme();
  return (
    <AQLink to={to}>
      <View
        style={{
          flexDirection: "row",
          gap: 12,
          paddingHorizontal: 20,
          paddingTop: 12,
          paddingBottom: 12,
          borderBottomWidth: 1,
          borderBottomColor: theme.colors.borderSubtle,
        }}
      >
        <Avatar src={avatarUrl} name={name} size={42} live={live} />
        <View style={{ flex: 1, minWidth: 0, gap: 8 }}>
          <View
            style={{
              flexDirection: "row",
              alignItems: "center",
              gap: 4,
              flexWrap: "nowrap",
            }}
          >
            <Text
              weight="semibold"
              numberOfLines={1}
              style={{ fontSize: 15, lineHeight: 20, flexShrink: 1 }}
            >
              {name}
            </Text>
            <Text
              numberOfLines={1}
              style={{
                fontSize: 15,
                lineHeight: 20,
                color: theme.colors.text3,
                flexShrink: 1,
              }}
            >
              @{handle}
            </Text>
            {createdAt && (
              <Text
                style={{
                  fontSize: 15,
                  lineHeight: 20,
                  color: theme.colors.text3,
                }}
              >
                · {relativeAge(createdAt)}
              </Text>
            )}
          </View>
          <Text style={{ fontSize: 15, lineHeight: 21 }} numberOfLines={3}>
            {text}
          </Text>
          <View
            style={{
              width: "100%",
              aspectRatio: 16 / 9,
              borderRadius: 12,
              overflow: "hidden",
              backgroundColor: theme.colors.surface2,
            }}
          >
            {imageUri ? (
              <Image
                source={{ uri: imageUri }}
                style={{ width: "100%", height: "100%" }}
                contentFit="cover"
              />
            ) : imageFallback ? (
              <Image
                source={imageFallback}
                style={{ width: "100%", height: "100%" }}
                contentFit="contain"
              />
            ) : null}
            <View
              style={{
                position: "absolute",
                top: 0,
                left: 0,
                right: 0,
                bottom: 0,
                alignItems: "center",
                justifyContent: "center",
                pointerEvents: "none",
              }}
            >
              <Play
                size={96}
                color={theme.colors.accent}
                fill={theme.colors.accent}
              />
            </View>
            {live && (
              <View style={{ position: "absolute", top: 10, left: 10 }}>
                <LiveBadge />
              </View>
            )}
            {corner && (
              <View style={{ position: "absolute", bottom: 8, right: 8 }}>
                {corner}
              </View>
            )}
          </View>
          {count !== undefined && (
            <View
              style={{
                flexDirection: "row",
                alignItems: "center",
                gap: 6,
                paddingTop: 2,
              }}
            >
              <Eye size={16} color={theme.colors.text3} />
              <Text
                tabular
                style={{
                  fontSize: 13,
                  lineHeight: 16,
                  color: theme.colors.text3,
                }}
              >
                {formatCount(count)}
                {countLabel ? ` ${countLabel}` : ""}
              </Text>
            </View>
          )}
        </View>
      </View>
    </AQLink>
  );
}

// The duration chip in a video's corner.
export function DurationChip({ children }: { children: string }) {
  return (
    <View
      style={{
        backgroundColor: scrims.dark,
        borderRadius: 4,
        paddingHorizontal: 4,
        paddingVertical: 1,
      }}
    >
      <Text tabular style={{ fontSize: 12, lineHeight: 16, color: "#fff" }}>
        {children}
      </Text>
    </View>
  );
}
