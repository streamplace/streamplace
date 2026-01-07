import {
  Button,
  Input,
  Slider,
  Text,
  View,
  zero,
} from "@streamplace/components";
import { useDanmuSettings } from "@streamplace/components/src/streamplace-store";
import { useTranslation } from "react-i18next";
import { Platform, ScrollView, useWindowDimensions } from "react-native";
import { SettingToggle } from "./components/setting-toggle";

export function DanmuCategorySettings() {
  const { t } = useTranslation("settings");
  const {
    danmuEnabled,
    danmuOpacity,
    danmuSpeed,
    danmuLaneCount,
    danmuMaxMessages,
    setDanmuEnabled,
    setDanmuOpacity,
    setDanmuSpeed,
    setDanmuLaneCount,
    setDanmuMaxMessages,
  } = useDanmuSettings();

  const { width } = useWindowDimensions();
  const isNarrowScreen = width < 550;

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <View style={{ maxWidth: 500, width: "100%" }}>
          <View style={[{ alignItems: "stretch" }, zero.gap.all[12]]}>
            {/* Enable/Disable Danmu */}
            <SettingToggle
              title={t("danmu-enabled")}
              description={t("danmu-enabled-description")}
              value={danmuEnabled}
              onValueChange={setDanmuEnabled}
            />

            {/* Opacity */}
            <View style={[zero.gap.all[6]]}>
              <View
                style={[
                  {
                    flexDirection: isNarrowScreen ? "column" : "row",
                    justifyContent: "space-between",
                    alignItems: "flex-start",
                  },
                  zero.gap.all[2],
                ]}
              >
                <View
                  style={[
                    {
                      flexDirection: "row",
                      alignItems: "center",
                      justifyContent: "flex-start",
                    },
                    zero.gap.all[2],
                  ]}
                >
                  <Text size="lg">{t("danmu-opacity")}:</Text>
                  <Input
                    value={String(danmuOpacity)}
                    onChangeText={(text) => {
                      const val = parseInt(text) || 0;
                      setDanmuOpacity(Math.min(100, Math.max(0, val)));
                    }}
                    containerStyle={{ width: 60 }}
                    keyboardType="number-pad"
                  />
                  <Text size="lg">%</Text>
                </View>
                <View
                  style={[
                    {
                      flexDirection: "row",
                      alignItems: "center",
                    },
                    zero.gap.all[2],
                  ]}
                >
                  {[0, 25, 50, 75, 100].map((value) => (
                    <Button
                      key={value}
                      onPress={() => setDanmuOpacity(value)}
                      variant={danmuOpacity === value ? "primary" : "secondary"}
                      size="pill"
                      width="min"
                    >
                      <Text size="lg">{value}</Text>
                    </Button>
                  ))}
                </View>
              </View>
              {Platform.OS === "web" && (
                <Slider.Root
                  value={[danmuOpacity] as any}
                  min={0}
                  max={100}
                  step={5}
                  onValueChange={(vals) => setDanmuOpacity(vals[0])}
                  style={{ width: "100%", height: 40 }}
                >
                  <Slider.Track
                    style={{
                      height: 4,
                      backgroundColor: "#374151",
                      borderRadius: 2,
                      width: "100%",
                    }}
                  >
                    <Slider.Range
                      style={{
                        height: 4,
                        borderRadius: 2,
                      }}
                    />
                    <Slider.Thumb
                      style={{
                        width: 20,
                        height: 20,
                        borderRadius: 10,
                        backgroundColor: "#3b82f6",
                        transform: [{ translateY: -8 }],
                      }}
                    />
                  </Slider.Track>
                </Slider.Root>
              )}
            </View>

            {/* Speed */}
            <View style={[zero.gap.all[6]]}>
              <View
                style={[
                  {
                    flexDirection: isNarrowScreen ? "column" : "row",
                    justifyContent: "space-between",
                    alignItems: "flex-start",
                  },
                  zero.gap.all[2],
                ]}
              >
                <View
                  style={[
                    {
                      flexDirection: "row",
                      alignItems: "center",
                    },
                    zero.gap.all[2],
                  ]}
                >
                  <Text size="lg">{t("danmu-speed")}: </Text>
                  <Input
                    value={String(danmuSpeed)}
                    onChangeText={(text) => {
                      const val = parseFloat(text) || 0;
                      setDanmuSpeed(Math.min(3, Math.max(0.1, val)));
                    }}
                    containerStyle={{ width: 60 }}
                    keyboardType="numeric"
                  />
                  <Text size="lg">×</Text>
                </View>
                <View
                  style={[
                    {
                      flexDirection: "row",
                      alignItems: "center",
                    },
                    zero.gap.all[2],
                  ]}
                >
                  {[
                    { label: "0.5×", value: 0.5 },
                    { label: "1×", value: 1 },
                    { label: "1.5×", value: 1.5 },
                    { label: "2×", value: 2 },
                  ].map(({ label, value }) => (
                    <Button
                      key={value}
                      onPress={() => setDanmuSpeed(value)}
                      variant={danmuSpeed === value ? "primary" : "secondary"}
                      size="pill"
                      width="min"
                    >
                      <Text size="lg">{label}</Text>
                    </Button>
                  ))}
                </View>
              </View>
              {Platform.OS === "web" && (
                <Slider.Root
                  value={[danmuSpeed] as any}
                  min={0.5}
                  max={2}
                  step={0.1}
                  onValueChange={(vals) => setDanmuSpeed(vals[0])}
                  style={{ width: "100%", height: 40 }}
                >
                  <Slider.Track
                    style={{
                      height: 4,
                      backgroundColor: "#374151",
                      borderRadius: 2,
                      width: "100%",
                    }}
                  >
                    <Slider.Range
                      style={{
                        height: 4,
                        borderRadius: 2,
                      }}
                    />
                    <Slider.Thumb
                      style={{
                        width: 20,
                        height: 20,
                        borderRadius: 10,
                        backgroundColor: "#3b82f6",
                        transform: [{ translateY: -8 }],
                      }}
                    />
                  </Slider.Track>
                </Slider.Root>
              )}
            </View>

            {/* Lane Count */}
            <View style={[zero.gap.all[6]]}>
              <View
                style={[
                  {
                    flexDirection: isNarrowScreen ? "column" : "row",
                    justifyContent: "space-between",
                    alignItems: "flex-start",
                  },
                  zero.gap.all[2],
                ]}
              >
                <View
                  style={[
                    {
                      flexDirection: "row",
                      alignItems: "center",
                    },
                    zero.gap.all[2],
                  ]}
                >
                  <Text size="lg">{t("danmu-lane-count")}: </Text>
                  <Input
                    value={String(danmuLaneCount)}
                    onChangeText={(text) => {
                      const val = parseInt(text) || 0;
                      setDanmuLaneCount(Math.min(20, Math.max(4, val)));
                    }}
                    containerStyle={{ width: 60 }}
                    keyboardType="number-pad"
                  />
                </View>
                <View
                  style={[
                    {
                      flexDirection: "row",
                      alignItems: "center",
                    },
                    zero.gap.all[2],
                  ]}
                >
                  {[6, 8, 10, 12, 15].map((value) => (
                    <Button
                      key={value}
                      onPress={() => setDanmuLaneCount(value)}
                      variant={
                        danmuLaneCount === value ? "primary" : "secondary"
                      }
                      size="pill"
                      width="min"
                    >
                      <Text size="lg">{value}</Text>
                    </Button>
                  ))}
                </View>
              </View>
              {Platform.OS === "web" && (
                <Slider.Root
                  value={[danmuLaneCount] as any}
                  min={4}
                  max={20}
                  step={1}
                  onValueChange={(vals) => setDanmuLaneCount(vals[0])}
                  style={{ width: "100%", height: 40 }}
                >
                  <Slider.Track
                    style={{
                      height: 4,
                      backgroundColor: "#374151",
                      borderRadius: 2,
                      width: "100%",
                    }}
                  >
                    <Slider.Range
                      style={{
                        height: 4,
                        borderRadius: 2,
                      }}
                    />
                    <Slider.Thumb
                      style={{
                        width: 20,
                        height: 20,
                        borderRadius: 10,
                        backgroundColor: "#3b82f6",
                        transform: [{ translateY: -8 }],
                      }}
                    />
                  </Slider.Track>
                </Slider.Root>
              )}
            </View>

            {/* Max Messages */}
            <View style={[zero.gap.all[6]]}>
              <View
                style={[
                  {
                    flexDirection: isNarrowScreen ? "column" : "row",
                    justifyContent: "space-between",
                    alignItems: "flex-start",
                  },
                  zero.gap.all[2],
                ]}
              >
                <View
                  style={[
                    {
                      flexDirection: "row",
                      alignItems: "center",
                    },
                    zero.gap.all[2],
                  ]}
                >
                  <Text size="lg">{t("danmu-max-messages")}: </Text>
                  <Input
                    value={String(danmuMaxMessages)}
                    onChangeText={(text) => {
                      const val = parseInt(text) || 0;
                      setDanmuMaxMessages(Math.min(200, Math.max(5, val)));
                    }}
                    containerStyle={{ width: 60 }}
                    keyboardType="number-pad"
                  />
                </View>
                <View
                  style={[
                    {
                      flexDirection: "row",
                      alignItems: "center",
                    },
                    zero.gap.all[2],
                  ]}
                >
                  {[10, 25, 50, 100].map((value) => (
                    <Button
                      key={value}
                      onPress={() => setDanmuMaxMessages(value)}
                      variant={
                        danmuMaxMessages === value ? "primary" : "secondary"
                      }
                      size="pill"
                      width="min"
                    >
                      <Text size="lg">{value}</Text>
                    </Button>
                  ))}
                </View>
              </View>
              {Platform.OS === "web" && (
                <Slider.Root
                  value={[danmuMaxMessages] as any}
                  min={5}
                  max={200}
                  step={5}
                  onValueChange={(vals) => setDanmuMaxMessages(vals[0])}
                  style={{ width: "100%", height: 40 }}
                >
                  <Slider.Track
                    style={{
                      height: 4,
                      backgroundColor: "#374151",
                      borderRadius: 2,
                      width: "100%",
                    }}
                  >
                    <Slider.Range
                      style={{
                        height: 4,
                        borderRadius: 2,
                      }}
                    />
                    <Slider.Thumb
                      style={{
                        width: 20,
                        height: 20,
                        borderRadius: 10,
                        backgroundColor: "#3b82f6",
                        transform: [{ translateY: -8 }],
                      }}
                    />
                  </Slider.Track>
                </Slider.Root>
              )}
            </View>
          </View>
        </View>
      </View>
    </ScrollView>
  );
}
