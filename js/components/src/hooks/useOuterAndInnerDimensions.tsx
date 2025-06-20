import { useCallback, useState } from "react";
import { LayoutChangeEvent } from "react-native";

export function useOuterAndInnerDimensions() {
  const [outerDimensions, setOuterDimensions] = useState({
    width: 0,
    height: 0,
  });
  const [innerDimensions, setInnerDimensions] = useState({
    width: 0,
    height: 0,
  });

  const onOuterLayout = useCallback((event: LayoutChangeEvent) => {
    const { width, height } = event.nativeEvent.layout;
    setOuterDimensions({ width, height });
  }, []);

  const onInnerLayout = useCallback((event: LayoutChangeEvent) => {
    const { width, height } = event.nativeEvent.layout;
    setInnerDimensions({ width, height });
  }, []);

  return {
    outerWidth: outerDimensions.width,
    outerHeight: outerDimensions.height,
    innerWidth: innerDimensions.width,
    innerHeight: innerDimensions.height,
    onOuterLayout,
    onInnerLayout,
  };
}
