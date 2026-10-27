import {
  IconButton,
  Player,
  PlayerUI,
  Text,
  useLivestream,
  useLivestreamStore,
  useTheme,
  zero,
} from "@streamplace/components";
import {
  borderAlphas,
  fontFamilies,
  scrims,
  surfaces,
  textAlphas,
} from "@streamplace/components/src/lib/theme/tokens";
import { DesktopUi } from "components/mobile/desktop-ui";
import { Image } from "expo-image";
import type { LiveDashboardStatus } from "hooks/useLiveDashboardStatus";
import { Eye, EyeOff } from "lucide-react-native";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { View } from "react-native";
import StreamScreen from "./live-selector";

const { flex, bg, r, borders, layout, p, text, w, h, mt } = zero;

interface StreamMonitorProps {
  userProfile?: any;
  status: LiveDashboardStatus;
  videoRef?: any;
}

function PreviewOverlay({ ended }: { ended: boolean }) {
  const { theme } = useTheme();
  const { t } = useTranslation("common");
  return (
    <View
      style={{
        position: "absolute",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        justifyContent: "flex-start",
        alignItems: "flex-start",
        pointerEvents: "none",
      }}
    >
      <View style={{ margin: 16, gap: 8, alignItems: "flex-start" }}>
        {/* Refined status pill — matches the connection HUD grammar. Pink
            (accent, not live-red) says "previewing, ready" rather than "on air". */}
        <View
          style={{
            flexDirection: "row",
            alignItems: "center",
            gap: 8,
            paddingHorizontal: 12,
            paddingVertical: 7,
            backgroundColor: scrims.dark,
            borderRadius: 999,
            borderWidth: 1,
            borderColor: borderAlphas.dark.strong,
          }}
        >
          <View
            style={{
              width: 7,
              height: 7,
              borderRadius: 999,
              backgroundColor: theme.colors.primary,
            }}
          />
          <Text
            style={{
              color: "white",
              fontSize: 12,
              fontFamily: fontFamilies.semiBold,
              fontWeight: "600",
              letterSpacing: 0.5,
            }}
          >
            PREVIEW
          </Text>
        </View>
        <View
          style={{
            backgroundColor: scrims.dark,
            borderRadius: 8,
            borderWidth: 1,
            borderColor: borderAlphas.dark.strong,
            paddingHorizontal: 12,
            paddingVertical: 8,
            maxWidth: 340,
          }}
        >
          <Text
            style={{
              color: "white",
              fontSize: 14,
              fontWeight: "600",
              lineHeight: 19,
            }}
          >
            {t(ended ? "creator-status-ended" : "creator-status-ready")}
          </Text>
          <Text
            style={{
              color: textAlphas.dark[2],
              fontSize: 12.5,
              fontWeight: "500",
              lineHeight: 17,
              marginTop: 2,
            }}
          >
            {t(
              ended
                ? "creator-status-ended-detail"
                : "creator-status-ready-detail",
            )}
          </Text>
        </View>
      </View>
    </View>
  );
}

export default function StreamMonitor({
  userProfile: propUserProfile,
  status,
  videoRef,
}: StreamMonitorProps) {
  const { theme } = useTheme();
  const { t } = useTranslation("common");
  const profile = useLivestreamStore((x) => x.profile);
  const segment = useLivestreamStore((x) => x.segment);
  const ls = useLivestream(true);

  // Use hook data primarily, fallback to props
  const canPreview =
    status.incoming === true || (status.incoming === null && !!segment);
  const userProfile = propUserProfile ?? profile;

  // State for hiding/showing stream and thumbnail rotation
  const [isStreamVisible, setIsStreamVisible] = useState(true);
  const [currentThumbnail, setCurrentThumbnail] = useState<null | string>(null);

  // Mock thumbnails - in a real implementation, these would come from your stream service
  const thumbnails = "/api/playback/" + profile?.did + "/stream.jpg";

  // Rotate thumbnails every 30 seconds when stream is hidden
  useEffect(() => {
    if (!isStreamVisible && canPreview) {
      const interval = setInterval(() => {
        setCurrentThumbnail(thumbnails + "?ts=" + String(Date.now()));
      }, 30000); // 30 seconds

      return () => clearInterval(interval);
    }
  }, [isStreamVisible, canPreview, thumbnails]);

  const getStatusDotColor = () => {
    if (status.phase === "reconnecting") return theme.colors.warning;
    if (status.phase === "live")
      return status.readingsStale ? theme.colors.warning : theme.colors.live;
    if (status.phase === "ready") return theme.colors.success;
    return theme.colors.textMuted;
  };

  const getStreamTitle = () => {
    if (!ls) {
      return (
        <Text
          style={[text.white, { fontSize: 14, fontWeight: "600" }]}
          numberOfLines={1}
          ellipsizeMode="tail"
        >
          {userProfile?.handle || userProfile?.displayName || "Not live"}
        </Text>
      );
    }
    return (
      <Text
        style={[text.white, { fontSize: 18, fontWeight: "600" }]}
        numberOfLines={1}
        ellipsizeMode="tail"
      >
        {ls?.record.title || "Stream Title"}
      </Text>
    );
  };

  return (
    <View
      style={[
        flex.values[2],
        { backgroundColor: surfaces.dark[1] },
        r.lg,
        borders.width.thin,
        { borderColor: borderAlphas.dark.strong },
        layout.flex.column,
        { overflow: "hidden" },
      ]}
    >
      <View
        style={[
          flex.values[1],
          layout.flex.center,
          { backgroundColor: surfaces.dark[1] },
        ]}
      >
        {canPreview && userProfile ? (
          isStreamVisible ? (
            <View
              style={{ position: "relative", width: "100%", height: "100%" }}
            >
              <Player
                src={userProfile.did}
                name={userProfile.handle}
                muted={true}
              >
                <DesktopUi />
                <PlayerUI.ViewerLoadingOverlay />
              </Player>
              {(!ls || ls.record.endedAt !== undefined) &&
                status.incoming !== null && (
                  <PreviewOverlay ended={status.phase === "ended"} />
                )}
            </View>
          ) : (
            <View
              style={[
                layout.flex.center,
                { position: "relative", width: "100%", height: "100%" },
              ]}
            >
              <Image
                source={{ uri: currentThumbnail || thumbnails }}
                contentFit="contain"
                style={{
                  width: "100%",
                  height: "100%",
                }}
              />
              <View
                style={{
                  position: "absolute",
                  top: 12,
                  left: 12,
                  backgroundColor: scrims.dark,
                  paddingHorizontal: 8,
                  paddingVertical: 4,
                  borderRadius: 4,
                }}
              >
                <Text style={[text.white, { fontSize: 12 }]}>
                  Thumbnail Preview
                </Text>
              </View>
            </View>
          )
        ) : status.incoming === null ? (
          <Text size="sm" color="muted">
            {t(`creator-status-${status.phase}`)}
          </Text>
        ) : (
          <StreamScreen route={profile?.did} />
        )}
      </View>
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
          p[4],
          borders.top.width.thin,
          { borderTopColor: borderAlphas.dark.strong },
        ]}
      >
        <View
          style={[
            layout.flex.row,
            layout.flex.alignCenter,
            { flex: 1, minWidth: 0, gap: 12 },
          ]}
        >
          <View style={{ flex: 1, minWidth: 0 }}>{getStreamTitle()}</View>
          <View
            style={[
              layout.flex.row,
              layout.flex.justify.end,
              layout.flex.alignCenter,
              { gap: 8, flexShrink: 0 },
            ]}
          >
            {canPreview && userProfile && (
              <IconButton
                size="sm"
                accessibilityLabel={
                  isStreamVisible ? "Hide preview" : "Show preview"
                }
                onPress={() => setIsStreamVisible(!isStreamVisible)}
              >
                {isStreamVisible ? (
                  <EyeOff size={16} color={textAlphas.dark[3]} />
                ) : (
                  <Eye size={16} color={textAlphas.dark[3]} />
                )}
              </IconButton>
            )}
            <View
              style={[layout.flex.row, layout.flex.alignCenter, { gap: 6 }]}
            >
              <View
                style={[
                  w[3],
                  h[3],
                  r.full,
                  { backgroundColor: getStatusDotColor() },
                ]}
              />
              <Text
                testID="creator-stream-status"
                style={{
                  color: textAlphas.dark[2],
                  fontSize: 12,
                  fontWeight: "600",
                  letterSpacing: 0.4,
                }}
              >
                {t(`creator-status-${status.phase}`)}
              </Text>
            </View>
          </View>
        </View>
      </View>
      {(status.phase === "reconnecting" ||
        (status.phase === "live" && status.readingsStale)) && (
        <Text
          size="xs"
          style={{
            color: theme.colors.textMuted,
            paddingHorizontal: theme.spacing[4],
            paddingBottom: theme.spacing[4],
          }}
        >
          {t(
            status.phase === "live"
              ? "creator-status-live-waiting-detail"
              : `creator-status-${status.phase}-detail`,
          )}
        </Text>
      )}
    </View>
  );
}
