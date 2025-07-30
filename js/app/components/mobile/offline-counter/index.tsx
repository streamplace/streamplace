import {
  Text,
  usePlayerStore,
  useSegment,
  View,
  zero,
} from "@streamplace/components";
import { useEffect, useState } from "react";

const { gap, h, layout, position, w } = zero;

interface OfflineCounterProps {
  isMobile?: boolean;
}

export function OfflineCounter({ isMobile = false }: OfflineCounterProps) {
  const offline = usePlayerStore((state) => state.offline);
  const segment = useSegment();

  // Live timer for offline overlay
  const [timeSinceLastSeen, setTimeSinceLastSeen] = useState("Unknown");

  useEffect(() => {
    if (!offline || !segment?.startTime) {
      setTimeSinceLastSeen("Unknown");
      return;
    }

    const updateTimer = () => {
      const now = new Date();
      const lastSeen = new Date(segment.startTime);
      const diffMs = now.getTime() - lastSeen.getTime();
      const diffMinutes = Math.floor(diffMs / 60000);
      const diffSeconds = Math.floor((diffMs % 60000) / 1000);

      if (diffMinutes > 0) {
        setTimeSinceLastSeen(`${diffMinutes}m ${diffSeconds}s ago`);
      } else {
        setTimeSinceLastSeen(`${diffSeconds}s ago`);
      }
    };

    // Update immediately
    updateTimer();

    // Update every second while offline
    const interval = setInterval(updateTimer, 1000);

    return () => clearInterval(interval);
  }, [offline, segment?.startTime]);

  if (!offline) return null;

  const titleFontSize = isMobile ? 24 : 32;
  const subtitleFontSize = isMobile ? 16 : 18;
  const descriptionFontSize = isMobile ? 14 : 16;
  const descriptionMarginTop = isMobile ? 8 : 12;
  const maxWidth = isMobile ? undefined : 400;

  return (
    <View
      style={[
        layout.position.absolute,
        layout.flex.center,
        h.percent[100],
        w.percent[100],
        {
          backgroundColor: "rgba(0, 0, 0, 0.85)",
          zIndex: 1000,
        },
      ]}
    >
      <View style={[layout.flex.column, layout.flex.center, gap.all[3]]}>
        <Text
          style={[
            {
              fontSize: titleFontSize,
              fontWeight: "700",
              color: "white",
              textAlign: "center",
            },
          ]}
        >
          Stream Offline
        </Text>
        <Text
          style={[
            {
              fontSize: subtitleFontSize,
              color: "rgba(255, 255, 255, 0.8)",
              textAlign: "center",
              fontVariant: ["tabular-nums"],
            },
          ]}
        >
          Last seen: {timeSinceLastSeen}
        </Text>
        <Text
          style={[
            {
              fontSize: descriptionFontSize,
              color: "rgba(255, 255, 255, 0.6)",
              textAlign: "center",
              marginTop: descriptionMarginTop,
              maxWidth,
            },
          ]}
        >
          The stream will resume automatically when the broadcaster returns
        </Text>
      </View>
    </View>
  );
}
