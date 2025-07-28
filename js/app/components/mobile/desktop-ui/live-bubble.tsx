import { Code, View, zero } from "@streamplace/components";

const { borders, gap, h, w, px, bg, text } = zero;

export function LiveBubble() {
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
          style={[
            text.white,
            {
              fontSize: 12,
              lineHeight: 8,
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
