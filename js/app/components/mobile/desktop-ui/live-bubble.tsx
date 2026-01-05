import { Code, useSegment, View, zero } from "@streamplace/components";
import { useMemo } from "react";

const { borders, gap, h, w, px, bg, text } = zero;

export function LiveBubble() {
  // are we actually live? (is the most recent segment <= 10 seconds old?)
  let seg = useSegment();

  let segDate = useMemo(() => {
    return seg?.startTime ? new Date(seg.startTime) : undefined;
  }, [seg?.startTime]);

  let isLive = useMemo(() => {
    return segDate && Date.now() - segDate.getTime() <= 10 * 1000;
  }, [segDate]);

  if (!isLive)
    return (
      <View style={[{ flexDirection: "row" }]}>
        <View
          style={[
            { flexDirection: "row", alignItems: "center" },
            gap.all[1],
            px[2],
            bg.gray[500],
            borders.color.gray[800],
            { paddingVertical: 3 },
          ]}
        >
          <Code
            size="xs"
            style={[
              text.white,
              {
                fontWeight: "600",
                letterSpacing: 2,
              },
            ]}
          >
            OFFLINE
          </Code>
        </View>
      </View>
    );

  return (
    <View style={[{ flexDirection: "row" }]}>
      <View
        style={[
          { flexDirection: "row", alignItems: "center" },
          gap.all[1],
          px[2],
          bg.destructive[500],
          borders.color.gray[800],
          { paddingVertical: 3 },
        ]}
      >
        <View style={[h[2], w[2], bg.white, { borderRadius: 999 }]} />
        <Code
          size="xs"
          style={[
            text.white,
            {
              fontWeight: "600",
              letterSpacing: 2,
            },
          ]}
        >
          LIVE
        </Code>
      </View>
    </View>
  );
}
