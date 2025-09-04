import {
  Car,
  ChevronDown,
  ChevronUp,
  Eye,
  EyeClosed,
  Monitor,
  Signal,
  Video,
  Zap,
} from "lucide-react-native";
import React, { useCallback, useEffect, useMemo, useState } from "react";
import { LayoutChangeEvent, Text, TouchableOpacity, View } from "react-native";
import Svg, { Path, Line as SvgLine, Text as SvgText } from "react-native-svg";
import {
  useLivestreamStore,
  useSegment,
  useViewers,
} from "../../livestream-store";
import * as zero from "../../ui";
import { InfoBox, InfoRow } from "../ui";

interface InformationWidgetProps {
  embedMode?: boolean;
  wideMode?: boolean;
  showChart?: boolean;
}

const BITRATE_HISTORY_LENGTH = 30;

const { bg, r, borders, px, py, text, layout, gap, flex, p } = zero;

export default function InformationWidget({
  embedMode = false,
  wideMode, // Optional override
  showChart = true,
}: InformationWidgetProps) {
  const [bitrateHistory, setBitrateHistory] = useState<number[]>(
    Array.from({ length: BITRATE_HISTORY_LENGTH }, () => 0),
  );
  const [showViewers, setShowViewers] = useState(false);
  const [componentWidth, setComponentWidth] = useState<number>(220);
  const [componentHeight, setComponentHeight] = useState<number>(400);
  const [streamStartTime, setStreamStartTime] = useState<Date | null>(null);
  const [layoutMeasured, setLayoutMeasured] = useState(false);
  const isWideMode =
    wideMode !== undefined ? wideMode : layoutMeasured && componentWidth > 400;

  const isCompactHeight = layoutMeasured && componentHeight < 350;

  const seg = useSegment();
  const livestream = useLivestreamStore((x) => x.livestream);
  const viewers = useViewers();

  const getBitrate = useCallback((): number => {
    if (!seg?.size || !seg?.duration) return 0;
    const kbps =
      (seg.size * 8) / ((seg.duration || 1000000000) / 1000000000) / 1000;
    return kbps;
  }, [seg?.size, seg?.duration]);

  const getMediaInfo = useMemo(() => {
    const videoTrack = seg?.video?.[0];
    const audioTrack = seg?.audio?.[0];
    return {
      resolution:
        videoTrack?.width && videoTrack?.height
          ? `${videoTrack.width}x${videoTrack.height}`
          : "Unknown",
      fps: videoTrack?.framerate
        ? `${(videoTrack.framerate.num / videoTrack.framerate.den).toFixed(
            2,
          )} FPS`
        : "Unknown",
      videoCodec: videoTrack?.codec
        ? videoTrack.codec.toUpperCase()
        : "Unknown",
    };
  }, [seg?.video, seg?.audio]);

  const currentBitrate = getBitrate();

  useEffect(() => {
    setBitrateHistory((prev) => [...prev.slice(1), currentBitrate]);
  }, [currentBitrate]);

  useEffect(() => {
    if (seg?.startTime && !streamStartTime) {
      setStreamStartTime(new Date(seg.startTime));
    }
  }, [seg?.startTime, streamStartTime]);

  const getBitrateStatus = (): "good" | "warning" | "error" | "neutral" => {
    if (currentBitrate > 2000) return "good";
    if (currentBitrate > 1000) return "warning";
    if (currentBitrate > 0) return "error";
    return "neutral";
  };

  const getConnectionStatus = (): "good" | "warning" | "error" | "neutral" => {
    if (!seg) return "error";
    if (currentBitrate > 1500) return "good";
    if (currentBitrate > 500) return "warning";
    return "error";
  };

  const avgBitrate =
    bitrateHistory.length > 0
      ? bitrateHistory.reduce((a, b) => a + b, 0) / bitrateHistory.length
      : 0;
  const peakBitrate = Math.max(...bitrateHistory, 0);
  const uptimeMinutes = streamStartTime
    ? Math.floor((Date.now() - streamStartTime.getTime()) / 60000)
    : 0;
  const estimatedLatency = seg?.duration
    ? Math.floor((seg.duration / 1000000000) * 2)
    : 0;

  const displayBitrate = `${(currentBitrate / 1000).toFixed(2)} Mbps`;
  const displayResolution = getMediaInfo.resolution;
  const displayFps = getMediaInfo.fps;
  const streamTitle = livestream?.record?.title || "Untitled Stream";
  const viewerCount = viewers ?? 0;

  const handleLayout = useCallback((event: LayoutChangeEvent) => {
    const { width, height } = event.nativeEvent.layout;
    console.log("InformationWidget onLayout - size:", `${width}x${height}`);
    if (width > 0 && height > 0) {
      setComponentWidth(width);
      setComponentHeight(height);
      setLayoutMeasured(true);
    }
  }, []);

  return (
    <View
      onLayout={handleLayout}
      style={[
        embedMode
          ? { backgroundColor: "rgba(23, 23, 23, 0.9)" }
          : bg.neutral[900],
        embedMode ? undefined : borders.width.thin,
        embedMode ? undefined : borders.color.neutral[700],
        r.lg,
        px[4],
        py[4],
        gap.all[6],
        flex.values[1],
        {
          minWidth: isWideMode ? 500 : 220,
          width: "100%",
        },
      ]}
    >
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
        ]}
      >
        <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[1]]}>
          <Text
            style={[text.white, { fontSize: 18, fontWeight: "700" }]}
            numberOfLines={1}
          >
            Stream Health
          </Text>
          <View
            style={[
              {
                width: 8,
                height: 8,
                borderRadius: 4,
                backgroundColor:
                  getConnectionStatus() === "good"
                    ? "#22c55e"
                    : getConnectionStatus() === "warning"
                      ? "#f59e0b"
                      : "#ef4444",
              },
            ]}
          />
        </View>
        <TouchableOpacity
          onPress={() => setShowViewers(!showViewers)}
          style={[
            layout.flex.column,
            layout.flex.alignCenter,
            gap.all[1],
            { minWidth: 120 },
          ]}
        >
          <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
            {showViewers ? (
              <Eye size={14} color="#9ca3af" />
            ) : (
              <EyeClosed size={14} color="#9ca3af" />
            )}
            <Text
              style={[
                text.white,
                { fontSize: 16, fontWeight: "600", textAlign: "center" },
              ]}
            >
              {showViewers ? `${viewerCount}` : "..."} viewer
              {showViewers && viewerCount !== 1 ? "s" : ""}
            </Text>
          </View>
        </TouchableOpacity>
      </View>

      {isWideMode ? (
        <View style={[gap.all[3]]}>
          <View style={[layout.flex.row, gap.all[2]]}>
            <InfoBox
              icon={Car}
              label="Bitrate"
              value={displayBitrate}
              status={getBitrateStatus()}
            />
            <InfoBox
              icon={Monitor}
              label="Resolution"
              value={displayResolution}
            />
            <InfoBox icon={Video} label="FPS" value={displayFps} />
          </View>

          {showChart && (
            <View style={[gap.all[2]]}>
              {!isCompactHeight && (
                <View
                  style={[
                    layout.flex.row,
                    layout.flex.spaceBetween,
                    layout.flex.alignCenter,
                  ]}
                >
                  <Text
                    style={[
                      text.gray[200],
                      { fontSize: 14, fontWeight: "600" },
                    ]}
                  >
                    Live Performance
                  </Text>
                  <View style={[layout.flex.row, gap.all[4]]}>
                    <View style={[layout.flex.alignCenter]}>
                      <Text
                        style={[
                          text.gray[400],
                          { fontSize: 11, fontWeight: "500" },
                        ]}
                      >
                        AVG
                      </Text>
                      <Text
                        style={[
                          text.white,
                          { fontSize: 13, fontWeight: "600" },
                        ]}
                      >
                        {avgBitrate > 0
                          ? `${(avgBitrate / 1000).toFixed(1)}M`
                          : "0M"}
                      </Text>
                    </View>
                    <View style={[layout.flex.alignCenter]}>
                      <Text
                        style={[
                          text.gray[400],
                          { fontSize: 11, fontWeight: "500" },
                        ]}
                      >
                        PEAK
                      </Text>
                      <Text
                        style={[
                          text.white,
                          { fontSize: 13, fontWeight: "600" },
                        ]}
                      >
                        {peakBitrate > 0
                          ? `${(peakBitrate / 1000).toFixed(1)}M`
                          : "0M"}
                      </Text>
                    </View>
                    <View style={[layout.flex.alignCenter]}>
                      <Text
                        style={[
                          text.gray[400],
                          { fontSize: 11, fontWeight: "500" },
                        ]}
                      >
                        CAPTURED
                      </Text>
                      <Text
                        style={[
                          text.white,
                          { fontSize: 13, fontWeight: "600" },
                        ]}
                      >
                        {uptimeMinutes > 60
                          ? `${Math.floor(uptimeMinutes / 60)}h ${uptimeMinutes % 60}m`
                          : `${uptimeMinutes}m`}
                      </Text>
                    </View>
                  </View>
                </View>
              )}

              <BitrateChart
                data={bitrateHistory}
                width={componentWidth - 40}
                height={120}
              />
            </View>
          )}
        </View>
      ) : (
        <View style={[gap.all[3]]}>
          {!isCompactHeight && (
            <TouchableOpacity
              onPress={() => setShowViewers(!showViewers)}
              style={[
                layout.flex.row,
                layout.flex.spaceBetween,
                layout.flex.alignCenter,
                py[2],
              ]}
            >
              <View
                style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}
              >
                <Eye size={16} color="#9ca3af" />
                <Text
                  style={[text.gray[300], { fontSize: 13, fontWeight: "500" }]}
                >
                  Viewers
                </Text>
              </View>
              <View
                style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}
              >
                <Text
                  style={[
                    showViewers ? text.green[400] : text.white,
                    { fontSize: 13, fontWeight: "600" },
                  ]}
                >
                  {showViewers ? `${viewerCount} watching` : "•••"}
                </Text>
                {showViewers ? (
                  <ChevronUp size={14} color="#9ca3af" />
                ) : (
                  <ChevronDown size={14} color="#9ca3af" />
                )}
              </View>
            </TouchableOpacity>
          )}

          {showChart && (
            <View style={[gap.all[2]]}>
              {!isCompactHeight && (
                <View
                  style={[
                    layout.flex.row,
                    layout.flex.spaceBetween,
                    layout.flex.alignCenter,
                  ]}
                >
                  <Text
                    style={[
                      text.gray[200],
                      { fontSize: 14, fontWeight: "600" },
                    ]}
                  >
                    Performance
                  </Text>
                  <Text
                    style={[
                      text.gray[400],
                      { fontSize: 11, fontWeight: "500" },
                    ]}
                  >
                    {avgBitrate > 0
                      ? `AVG ${(avgBitrate / 1000).toFixed(1)}M`
                      : "No data"}
                  </Text>
                </View>
              )}
              <BitrateChart
                data={bitrateHistory}
                width={componentWidth - 40}
                height={isCompactHeight ? 80 : 120}
              />
            </View>
          )}

          <View style={[gap.all[1]]}>
            <InfoRow
              icon={Signal}
              label="Connection"
              value={
                getConnectionStatus() === "good"
                  ? "Excellent"
                  : getConnectionStatus() === "warning"
                    ? "Good"
                    : "Poor"
              }
              status={getConnectionStatus()}
            />
            <View style={[gap.all[1], layout.flex.row]}>
              <InfoBox
                icon={Zap}
                label="Bitrate"
                value={displayBitrate}
                status={getBitrateStatus()}
              />
              <InfoBox icon={Video} label="FPS" value={displayFps} />
            </View>
          </View>
        </View>
      )}
    </View>
  );
}

function BitrateChart({
  data,
  width,
  height,
}: {
  data: number[];
  width: number;
  height: number;
}) {
  const maxDataValue = Math.max(...data, 1);
  const minDataValue = Math.min(...data);
  const getSmartRange = (max: number) => {
    if (max <= 1000) return { min: 0, max: 1000, step: 500 };
    if (max <= 2000) return { min: 1000, max: 2000, step: 1000 };
    if (max <= 7000) return { min: 4000, max: 7000, step: 1500 };
    if (max <= 10000) return { min: 4000, max: 10000, step: 5000 };

    const roundedMax = Math.ceil(max / 5000) * 5000;
    return { min: 0, max: roundedMax, step: roundedMax / 2 };
  };

  const { min: minValue, max: maxValue, step } = getSmartRange(maxDataValue);
  const range = maxValue - minValue;

  const chartWidth = width - 40;
  const chartStartX = 40;
  const verticalPadding = 10;
  const chartHeight = height - verticalPadding * 2;

  const points = data
    .map((value, index) => {
      const x = chartStartX + (index / (data.length - 1)) * chartWidth;
      // Clamp value to chart range and plot against the smart scale
      const clampedValue = Math.max(minValue, Math.min(maxValue, value));
      const y =
        verticalPadding +
        chartHeight -
        ((clampedValue - minValue) / range) * chartHeight;
      return `${x},${y}`;
    })
    .join(" ");

  const pathData = `M ${points.replace(/ /g, " L ")}`;
  const ticks = [
    { value: minValue, y: height - verticalPadding },
    { value: minValue + step, y: height / 2 },
    { value: maxValue, y: verticalPadding },
  ];

  return (
    <View style={{ height, width, marginVertical: 8 }}>
      <Svg width={width} height={height}>
        {ticks.map((tick, index) => (
          <React.Fragment key={index}>
            <SvgLine
              x1={chartStartX}
              y1={tick.y}
              x2={width}
              y2={tick.y}
              stroke="rgba(255,255,255,0.1)"
              strokeWidth="1"
            />
            <SvgText
              x="35"
              y={tick.y + 4}
              fontSize={10}
              fontFamily="sans-serif"
              fill="#9ca3af"
              textAnchor="end"
            >
              {(tick.value / 1000).toLocaleString()}
            </SvgText>
          </React.Fragment>
        ))}
        <SvgText
          x={12}
          y={height / 2}
          transform={`rotate(-90, 12, ${height / 2})`}
          fontSize={10}
          fontFamily="sans-serif"
          fill="#9ca3af"
          textAnchor="middle"
        >
          mbits/s
        </SvgText>
        <Path
          d={pathData}
          stroke="#22c55e"
          strokeWidth="2"
          fill="none"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </Svg>
    </View>
  );
}
