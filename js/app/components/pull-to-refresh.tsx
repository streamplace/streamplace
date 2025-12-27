import { useTheme } from "@streamplace/components";
import { Check, Loader } from "lucide-react-native";
import { useEffect, useRef, useState } from "react";
import {
  Animated,
  Easing,
  NativeScrollEvent,
  NativeSyntheticEvent,
  Platform,
  ScrollView,
  ScrollViewProps,
  View,
} from "react-native";

const PULL_THRESHOLD = 80;

function RefreshIndicator({
  pullY,
  refreshing,
  top,
  onDoneRefreshing,
}: {
  pullY: Animated.Value;
  refreshing: boolean;
  top: number;
  onDoneRefreshing?: () => void;
}) {
  const rotation = useRef(new Animated.Value(0)).current;
  const fadeOut = useRef(new Animated.Value(1)).current;
  const loopRef = useRef<Animated.CompositeAnimation | null>(null);
  const prevRefreshing = useRef(refreshing);
  const checkOpacity = useRef(new Animated.Value(0)).current;
  const { theme } = useTheme();

  useEffect(() => {
    if (refreshing) {
      fadeOut.setValue(1);
      checkOpacity.setValue(0);
      loopRef.current = Animated.loop(
        Animated.timing(rotation, {
          toValue: 1,
          duration: 800,
          easing: Easing.linear,
          useNativeDriver: true,
        }),
      );
      loopRef.current.start();
    } else if (prevRefreshing.current) {
      loopRef.current?.stop();
      Animated.timing(fadeOut, {
        toValue: 0,
        duration: 100,
        useNativeDriver: true,
      }).start(() => {
        rotation.setValue(0);
        fadeOut.setValue(1);
        Animated.sequence([
          Animated.delay(300),
          Animated.timing(checkOpacity, {
            toValue: 1,
            duration: 300,
            useNativeDriver: true,
          }),
          Animated.delay(800),
          Animated.timing(checkOpacity, {
            toValue: 0,
            duration: 300,
            useNativeDriver: true,
          }),
        ]).start(() => {
          onDoneRefreshing?.();
        });
      });
    }
    prevRefreshing.current = refreshing;
  }, [refreshing]);

  const spin = rotation.interpolate({
    inputRange: [0, 1],
    outputRange: ["0deg", "360deg"],
  });

  const pullAmount = pullY.interpolate({
    inputRange: [-PULL_THRESHOLD, 0],
    outputRange: [PULL_THRESHOLD, 0],
    extrapolate: "clamp",
  });

  const pullOpacity = pullY.interpolate({
    inputRange: [-PULL_THRESHOLD, -20, 0],
    outputRange: [1, 0.4, 0],
    extrapolate: "clamp",
  });

  const progressRotation = pullY.interpolate({
    inputRange: [-PULL_THRESHOLD, 0],
    outputRange: ["360deg", "0deg"],
    extrapolate: "clamp",
  });

  return (
    <Animated.View
      pointerEvents="none"
      style={{
        position: "absolute",
        top,
        left: 0,
        right: 0,
        alignItems: "center",
        zIndex: 100,
        transform: [{ translateY: refreshing ? 0 : pullAmount }],
      }}
    >
      <Animated.View
        style={{
          position: "absolute",
          // opposite of check opacity so it fades out as the check fades in
          opacity: pullOpacity,
        }}
      >
        <Animated.View
          style={{
            opacity: Animated.subtract(1, checkOpacity),
            transform: [{ rotate: refreshing ? spin : progressRotation }],
          }}
        >
          <Loader color={theme.colors.mutedForeground} size={32} />
        </Animated.View>
        <Animated.View style={{ position: "absolute", opacity: checkOpacity }}>
          <Check color={theme.colors.success} size={32} />
        </Animated.View>
      </Animated.View>
    </Animated.View>
  );
}

type PullToRefreshScrollViewProps = ScrollViewProps & {
  refreshing: boolean;
  onRefresh: () => void;
  onDoneRefreshing?: () => void;
  indicatorTop?: number;
};

export default function PullToRefreshScrollView({
  refreshing,
  onRefresh,
  onDoneRefreshing,
  indicatorTop = 120,
  children,
  ...scrollViewProps
}: PullToRefreshScrollViewProps) {
  const pullY = useRef(new Animated.Value(0)).current;
  const triggered = useRef(false);
  const scrollRef = useRef<ScrollView>(null);
  const isDragging = useRef(false);
  const currentY = useRef(0);
  const [contentInset, setContentInset] = useState(0);
  const prevRefreshing = useRef(refreshing);

  const snapToRefreshing = () => {
    setContentInset(PULL_THRESHOLD);
    scrollRef.current?.scrollTo({ y: -PULL_THRESHOLD, animated: true });
  };

  const snapBack = () => {
    scrollRef.current?.scrollTo({ y: 0, animated: true });
    setTimeout(() => setContentInset(0), 300);
  };

  useEffect(() => {
    if (refreshing && !prevRefreshing.current) {
      if (!isDragging.current) snapToRefreshing();
    } else if (!refreshing && prevRefreshing.current) {
      if (!isDragging.current) snapBack();
    }
    prevRefreshing.current = refreshing;
  }, [refreshing]);

  return (
    <View style={{ flex: 1, position: "relative" }}>
      {Platform.OS !== "web" && (
        <RefreshIndicator
          pullY={pullY}
          refreshing={refreshing}
          top={indicatorTop}
          onDoneRefreshing={onDoneRefreshing}
        />
      )}
      <Animated.ScrollView
        {...scrollViewProps}
        ref={scrollRef as any}
        bounces={true}
        scrollEventThrottle={16}
        contentInset={{ top: contentInset }}
        onScroll={Animated.event(
          [{ nativeEvent: { contentOffset: { y: pullY } } }],
          {
            useNativeDriver: true,
            listener: (e: NativeSyntheticEvent<NativeScrollEvent>) => {
              const y = e.nativeEvent.contentOffset.y;
              currentY.current = y;
              if (y < -PULL_THRESHOLD && !refreshing && !triggered.current) {
                triggered.current = true;
                onRefresh();
              } else if (y >= 0) {
                triggered.current = false;
              }
              scrollViewProps.onScroll?.(e);
            },
          },
        )}
        onScrollBeginDrag={(e) => {
          isDragging.current = true;
          scrollViewProps.onScrollBeginDrag?.(e);
        }}
        onScrollEndDrag={(e) => {
          isDragging.current = false;
          if (refreshing) snapToRefreshing();
          else if (currentY.current < 0) snapBack();
          scrollViewProps.onScrollEndDrag?.(e);
        }}
      >
        {children}
      </Animated.ScrollView>
    </View>
  );
}
