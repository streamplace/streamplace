import { Dimensions } from "react-native";
import { usePlayerStore } from "../..";

/**
 * usePlayerDimensions
 * Returns player and device dimensions, and whether the player aspect ratio is greater than the device's.
 */
export function usePlayerDimensions() {
  const { width, height } = Dimensions.get("window");
  const pHeight = Number(usePlayerStore((x) => x.playerHeight)) || 0;
  const pWidth = Number(usePlayerStore((x) => x.playerWidth)) || 0;

  const isPlayerRatioGreater =
    pHeight > 0 && pWidth > 0 ? pWidth / pHeight > width / height : false;

  return {
    width,
    height,
    pWidth,
    pHeight,
    isPlayerRatioGreater,
  };
}
