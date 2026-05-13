import { X } from "lucide-react-native";
import React, { useRef, useState } from "react";
import { Platform, Pressable } from "react-native";
import type { LeafletFlatBlock, PanelRange } from "../../lib/leaflet-to-bio";
import { useTheme } from "../../lib/theme/theme";
import { Text } from "../ui/text";
import { View } from "../ui/view";

export const PANEL_RANGE_COLORS = [
  "#e74c3c",
  "#3498db",
  "#2ecc71",
  "#f39c12",
  "#9b59b6",
  "#1abc9c",
  "#e91e63",
  "#f1c40f",
];

const BLOCK_TYPE_LABELS: Record<string, string> = {
  text: "text",
  header: "header",
  image: "image",
  orderedList: "list",
  unorderedList: "list",
  blockquote: "quote",
  divider: "divider",
  link: "link",
  socialLinks: "socials",
  livestream: "stream",
  schedule: "schedule",
  bskyPost: "post",
  embed: "embed",
  button: "link",
  horizontalRule: "divider",
  website: "embed",
  iframe: "embed",
};

interface LeafletPanelRangeSelectorProps {
  blocks: LeafletFlatBlock[];
  ranges: PanelRange[];
  onRangesChange: (ranges: PanelRange[]) => void;
}

export function LeafletPanelRangeSelector({
  blocks,
  ranges,
  onRangesChange,
}: LeafletPanelRangeSelectorProps) {
  const { theme } = useTheme();
  const [pendingStart, setPendingStart] = useState<number | null>(null);
  const [hoverIdx, setHoverIdx] = useState<number | null>(null);
  const isPointerDownRef = useRef(false);
  const isDraggingRef = useRef(false);

  const nextColor =
    PANEL_RANGE_COLORS[ranges.length % PANEL_RANGE_COLORS.length];

  function getRangeForBlock(idx: number): PanelRange | null {
    return ranges.find((r) => idx >= r.startIdx && idx <= r.endIdx) ?? null;
  }

  function pendingLo(): number | null {
    if (pendingStart === null) return null;
    const end = hoverIdx ?? pendingStart;
    return Math.min(pendingStart, end);
  }

  function pendingHi(): number | null {
    if (pendingStart === null) return null;
    const end = hoverIdx ?? pendingStart;
    return Math.max(pendingStart, end);
  }

  function inPending(idx: number): boolean {
    const lo = pendingLo();
    const hi = pendingHi();
    return lo !== null && hi !== null && idx >= lo && idx <= hi;
  }

  function completeRange(lo: number, hi: number) {
    const overlaps = ranges.some((r) => !(hi < r.startIdx || lo > r.endIdx));
    if (!overlaps) {
      onRangesChange([
        ...ranges,
        { startIdx: lo, endIdx: hi, color: nextColor },
      ]);
    }
    setPendingStart(null);
    setHoverIdx(null);
  }

  function removeRange(range: PanelRange) {
    onRangesChange(ranges.filter((r) => r !== range));
    setPendingStart(null);
  }

  function handlePress(idx: number) {
    if (isDraggingRef.current) return;

    const existing = getRangeForBlock(idx);
    if (existing) {
      removeRange(existing);
      return;
    }

    if (pendingStart === null) {
      setPendingStart(idx);
    } else if (pendingStart === idx) {
      setPendingStart(null);
      setHoverIdx(null);
    } else {
      const lo = Math.min(pendingStart, idx);
      const hi = Math.max(pendingStart, idx);
      completeRange(lo, hi);
    }
  }

  // Pointer events for drag (web only — silently ignored on native)
  function pointerProps(idx: number) {
    if (Platform.OS !== "web") return {};
    return {
      onPointerDown: () => {
        isPointerDownRef.current = true;
        isDraggingRef.current = false;
        if (getRangeForBlock(idx)) return;
        setPendingStart(idx);
        setHoverIdx(idx);
      },
      onPointerEnter: () => {
        if (!isPointerDownRef.current) return;
        isDraggingRef.current = true;
        setHoverIdx(idx);
      },
      onPointerUp: () => {
        isPointerDownRef.current = false;
        if (isDraggingRef.current && pendingStart !== null) {
          const lo = Math.min(pendingStart, idx);
          const hi = Math.max(pendingStart, idx);
          isDraggingRef.current = false;
          completeRange(lo, hi);
        }
      },
    };
  }

  const pageBreaks = new Set<number>();
  blocks.forEach((b, i) => {
    if (i > 0 && b.pageIdx !== blocks[i - 1].pageIdx) {
      pageBreaks.add(i);
    }
  });

  return (
    <View>
      {pendingStart !== null && (
        <View
          direction="row"
          align="center"
          style={{
            paddingHorizontal: 12,
            paddingVertical: 8,
            marginBottom: 4,
            gap: 8,
          }}
        >
          <View
            style={{
              width: 10,
              height: 10,
              borderRadius: 5,
              backgroundColor: nextColor,
            }}
          />
          <Text size="sm" color="muted" style={{ flex: 1 }}>
            Click another block to complete this panel range
          </Text>
          <Pressable
            onPress={() => {
              setPendingStart(null);
              setHoverIdx(null);
            }}
          >
            <Text size="sm" color="muted">
              Cancel
            </Text>
          </Pressable>
        </View>
      )}

      {pendingStart === null && ranges.length === 0 && (
        <Text
          size="sm"
          color="muted"
          style={{ paddingHorizontal: 12, paddingVertical: 8, marginBottom: 4 }}
        >
          Click the first block of a panel range to start selecting
        </Text>
      )}

      {blocks.map((block, i) => {
        const range = getRangeForBlock(block.idx);
        const isPending = inPending(block.idx);
        const isAnchor = block.idx === pendingStart;
        const color = range?.color ?? (isPending ? nextColor : null);
        const isFirst = range ? block.idx === range.startIdx : isAnchor;
        const isLast = range
          ? block.idx === range.endIdx
          : isPending && block.idx === (hoverIdx ?? pendingStart);

        const showPageBreak = pageBreaks.has(i);

        return (
          <React.Fragment key={block.idx}>
            {showPageBreak && (
              <View
                style={{
                  flexDirection: "row",
                  alignItems: "center",
                  gap: 8,
                  paddingHorizontal: 12,
                  paddingVertical: 6,
                }}
              >
                <View
                  style={{
                    flex: 1,
                    height: 1,
                    backgroundColor: theme.colors.border,
                  }}
                />
                <Text size="xs" color="muted">
                  page {block.pageIdx + 1}
                </Text>
                <View
                  style={{
                    flex: 1,
                    height: 1,
                    backgroundColor: theme.colors.border,
                  }}
                />
              </View>
            )}
            <Pressable
              onPress={() => handlePress(block.idx)}
              {...(pointerProps(block.idx) as any)}
              style={({ pressed }) => ({
                flexDirection: "row",
                alignItems: "center",
                paddingVertical: 6,
                paddingRight: 12,
                opacity:
                  !color && pendingStart !== null && !isPending ? 0.4 : 1,
                backgroundColor: pressed
                  ? theme.colors.muted
                  : color
                    ? `${color}18`
                    : "transparent",
                userSelect: "none",
              })}
            >
              {/* Range color bar */}
              <View
                style={{
                  width: 4,
                  alignSelf: "stretch",
                  marginRight: 10,
                  marginLeft: 4,
                  borderTopLeftRadius: isFirst ? 3 : 0,
                  borderTopRightRadius: isFirst ? 3 : 0,
                  borderBottomLeftRadius: isLast ? 3 : 0,
                  borderBottomRightRadius: isLast ? 3 : 0,
                  backgroundColor: color ?? "transparent",
                }}
              />

              {/* Block type badge */}
              <Text
                size="xs"
                color="muted"
                style={{
                  minWidth: 52,
                  textAlign: "right",
                  marginRight: 10,
                  opacity: 0.7,
                }}
              >
                {BLOCK_TYPE_LABELS[block.blockType] ?? block.blockType}
              </Text>

              {/* Label */}
              <Text size="sm" style={{ flex: 1 }} numberOfLines={1}>
                {block.label}
              </Text>

              {/* Remove button for completed ranges */}
              {range && block.idx === range.startIdx && (
                <Pressable
                  onPress={(e) => {
                    e.stopPropagation?.();
                    removeRange(range);
                  }}
                  style={{
                    marginLeft: 8,
                    width: 20,
                    height: 20,
                    borderRadius: 10,
                    backgroundColor: range.color,
                    alignItems: "center",
                    justifyContent: "center",
                  }}
                  hitSlop={8}
                >
                  <X size={10} color="#fff" />
                </Pressable>
              )}
            </Pressable>
          </React.Fragment>
        );
      })}

      {ranges.length > 0 && (
        <View
          direction="row"
          style={{
            flexWrap: "wrap",
            gap: 6,
            paddingHorizontal: 12,
            paddingTop: 12,
            paddingBottom: 4,
          }}
        >
          {ranges.map((r, i) => (
            <View
              key={i}
              direction="row"
              align="center"
              style={{
                backgroundColor: `${r.color}30`,
                borderRadius: 12,
                paddingHorizontal: 8,
                paddingVertical: 3,
                gap: 4,
              }}
            >
              <View
                style={{
                  width: 8,
                  height: 8,
                  borderRadius: 4,
                  backgroundColor: r.color,
                }}
              />
              <Text size="xs">
                Panel {i + 1} ({r.endIdx - r.startIdx + 1} blocks)
              </Text>
            </View>
          ))}
        </View>
      )}
    </View>
  );
}
