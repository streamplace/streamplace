import { useCallback, useEffect, useMemo, useState } from "react";
import { LayoutChangeEvent } from "react-native";
import { PlaceStreamBioLayoutsPanels } from "streamplace";
import { View } from "../ui/view";

const COLUMN_WIDTH = 350;
const GAP = 16;

export function PanelsLayout({
  panels,
  renderPanel,
}: {
  panels: PlaceStreamBioLayoutsPanels.Panel[];
  renderPanel: (
    panel: PlaceStreamBioLayoutsPanels.Panel,
    index: number,
  ) => React.ReactNode;
}) {
  const [containerWidth, setContainerWidth] = useState(0);
  const [heights, setHeights] = useState<(number | null)[]>(() =>
    new Array(panels.length).fill(null),
  );

  useEffect(() => {
    setHeights(new Array(panels.length).fill(null));
  }, [panels.length]);

  const allMeasured = heights.every((h) => h !== null) && containerWidth > 0;

  const numColumns = useMemo(() => {
    if (containerWidth === 0) return 1;
    return Math.max(
      1,
      Math.floor((containerWidth + GAP) / (COLUMN_WIDTH + GAP)),
    );
  }, [containerWidth]);

  const { positions, containerHeight } = useMemo(() => {
    if (!allMeasured) return { positions: [], containerHeight: 0 };

    const columnTops = new Array(numColumns).fill(0);
    const positions: { x: number; y: number }[] = [];
    const totalColumnsWidth =
      numColumns * COLUMN_WIDTH + (numColumns - 1) * GAP;
    const offset = Math.max(0, (containerWidth - totalColumnsWidth) / 2);

    for (let i = 0; i < panels.length; i++) {
      const shortestCol = columnTops.indexOf(Math.min(...columnTops));
      positions.push({
        x: offset + shortestCol * (COLUMN_WIDTH + GAP),
        y: columnTops[shortestCol],
      });
      columnTops[shortestCol] += heights[i]! + GAP;
    }

    return {
      positions,
      containerHeight: Math.max(...columnTops) - GAP,
    };
  }, [allMeasured, numColumns, heights, panels.length, containerWidth]);

  const handleContainerLayout = useCallback((e: LayoutChangeEvent) => {
    setContainerWidth(e.nativeEvent.layout.width);
  }, []);

  const handlePanelLayout = useCallback(
    (index: number) => (e: LayoutChangeEvent) => {
      setHeights((prev) => {
        const next = [...prev];
        next[index] = e.nativeEvent.layout.height;
        return next;
      });
    },
    [],
  );

  if (!allMeasured) {
    return (
      <View
        fullWidth
        onLayout={handleContainerLayout}
        style={{ position: "relative", overflow: "hidden" }}
      >
        {containerWidth > 0 && (
          <View
            direction="row"
            align="start"
            style={{
              position: "absolute",
              top: -99999,
              left: -99999,
              flexWrap: "wrap",
              gap: GAP,
            }}
          >
            {panels.map((panel, panelIdx) => (
              <View
                key={panelIdx}
                style={{ width: COLUMN_WIDTH }}
                onLayout={handlePanelLayout(panelIdx)}
              >
                {renderPanel(panel, panelIdx)}
              </View>
            ))}
          </View>
        )}
      </View>
    );
  }

  return (
    <View
      onLayout={handleContainerLayout}
      style={{ height: containerHeight, position: "relative", width: "100%" }}
    >
      {panels.map((panel, panelIdx) => (
        <View
          key={panelIdx}
          style={{
            position: "absolute",
            left: positions[panelIdx].x,
            top: positions[panelIdx].y,
            width: COLUMN_WIDTH,
          }}
        >
          {renderPanel(panel, panelIdx)}
        </View>
      ))}
    </View>
  );
}
