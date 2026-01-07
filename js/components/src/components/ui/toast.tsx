import { Portal } from "@rn-primitives/portal";
import { CheckCircle, Info, X, XCircle } from "lucide-react-native";
import { useEffect, useRef, useState } from "react";
import {
  Platform,
  Pressable,
  StyleSheet,
  useWindowDimensions,
  View,
  ViewStyle,
} from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import Animated, {
  cancelAnimation,
  Easing,
  runOnJS,
  useAnimatedProps,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { Circle, Svg } from "react-native-svg";
import { useTheme } from "../../ui";
import { Button } from "./button";
import { Text } from "./text";

type Position =
  | "auto"
  | "top-left"
  | "top-center"
  | "top-right"
  | "bottom-left"
  | "bottom-center"
  | "bottom-right";

type ToastConfig = {
  title?: string;
  description?: string;
  duration?: number;
  actionLabel?: string;
  onAction?: () => void;
  onClose?: () => void;
  variant?: "default" | "success" | "error" | "info";
  render?: (props: {
    close: () => void;
    action?: () => void;
  }) => React.ReactNode;
  position?: Position;
  iconLeft?: React.ComponentType<any>;
  iconRight?: React.ComponentType<any>;
  onToastPress?: () => void;
};

type ToastState = {
  id: string;
  open: boolean;
  title?: string;
  description?: string;
  duration: number;
  actionLabel?: string;
  onAction?: () => void;
  onClose?: () => void;
  variant?: "default" | "success" | "error" | "info";
  render?: (props: {
    close: () => void;
    action?: () => void;
  }) => React.ReactNode;
  position: Position;
  iconLeft?: React.ComponentType<any>;
  iconRight?: React.ComponentType<any>;
  onToastPress?: () => void;
};

class ToastManager {
  private listeners: Set<(state: ToastState[]) => void> = new Set();
  private toasts: ToastState[] = [];
  private toastHeights: Map<string, number> = new Map();

  private hoverListeners: Set<(isHovered: boolean) => void> = new Set();
  private isHovered: boolean = false;

  show(config: ToastConfig) {
    const toast: ToastState = {
      id: Math.random().toString(36).slice(2, 12),
      open: true,
      title: config.title,
      description: config.description,
      duration: config.duration ?? 3,
      actionLabel: config.actionLabel,
      onAction: config.onAction,
      onClose: config.onClose,
      variant: config.variant ?? "default",
      render: config.render,
      position: config.position ?? "auto",
      iconLeft: config.iconLeft,
      iconRight: config.iconRight,
      onToastPress: config.onToastPress,
    };

    this.toasts = [...this.toasts, toast];
    this.notifyListeners();
  }

  getToasts() {
    return this.toasts;
  }

  hide(id: string) {
    const toast = this.toasts.find((t) => t.id === id);
    if (toast?.onClose) {
      toast.onClose();
    }
    this.toasts = this.toasts.map((toast) =>
      toast.id === id ? { ...toast, open: false } : toast,
    );
    this.notifyListeners();

    setTimeout(() => {
      this.toasts = this.toasts.filter((toast) => toast.id !== id);
      this.notifyListeners();
    }, 500);
  }

  subscribe(listener: (state: ToastState[]) => void) {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  subscribeHover(listener: (isHovered: boolean) => void) {
    this.hoverListeners.add(listener);
    return () => {
      this.hoverListeners.delete(listener);
    };
  }

  setHovered(hovered: boolean) {
    this.isHovered = hovered;
    this.notifyHoverListeners();
  }

  updateToastHeight(id: string, height: number) {
    this.toastHeights.set(id, height);
  }

  getToastHeight(id: string): number {
    return this.toastHeights.get(id) || 100; // Default fallback height
  }

  private notifyListeners() {
    this.listeners.forEach((listener) => listener(this.toasts));
  }

  private notifyHoverListeners() {
    this.hoverListeners.forEach((listener) => listener(this.isHovered));
  }
}

const toastManager = new ToastManager();

export const toast = {
  show: (
    title: string,
    description?: string,
    options?: {
      duration?: number;
      actionLabel?: string;
      onAction?: () => void;
      onClose?: () => void;
      variant?: "default" | "success" | "error" | "info";
      position?: Position;
      iconLeft?: React.ComponentType<any>;
      iconRight?: React.ComponentType<any>;
      onToastPress?: () => void;
    },
  ) => {
    toastManager.show({
      title,
      description,
      ...options,
    });
  },
  hide: (id?: string) => {
    if (id) {
      toastManager.hide(id);
    } else {
      const toasts = toastManager.getToasts();
      if (toasts.length > 0) {
        toastManager.hide(toasts[toasts.length - 1].id);
      }
    }
  },
  showManual: (
    render: (props: {
      close: () => void;
      action?: () => void;
    }) => React.ReactNode,
    options?: {
      duration?: number;
      actionLabel?: string;
      onAction?: () => void;
      onClose?: () => void;
      variant?: "default" | "success" | "error" | "info";
      position?: Position;
      iconLeft?: React.ComponentType<any>;
      iconRight?: React.ComponentType<any>;
      onToastPress?: () => void;
    },
  ) => {
    toastManager.show({
      render,
      ...options,
    });
  },
};

export function useToast() {
  const [toasts, setToasts] = useState<ToastState[]>([]);

  useEffect(() => {
    return toastManager.subscribe(setToasts);
  }, []);

  return {
    toasts,
    ...toast,
  };
}

export function ToastProvider() {
  const [toasts, setToasts] = useState<ToastState[]>([]);

  useEffect(() => {
    return toastManager.subscribe(setToasts);
  }, []);

  const toastsByPosition = toasts.reduce(
    (acc, toast) => {
      const { position } = toast;
      if (!acc[position]) {
        acc[position] = [];
      }
      acc[position].push(toast);
      return acc;
    },
    {} as Record<Position, ToastState[]>,
  );

  return (
    <>
      {Object.entries(toastsByPosition).map(([position, toasts]) => (
        <ToastContainer
          key={position}
          position={position as Position}
          toasts={toasts}
        />
      ))}
    </>
  );
}

function ToastContainer({
  position = "auto",
  toasts,
}: {
  position?: Position;
  toasts: ToastState[];
}) {
  const insets = useSafeAreaInsets();
  const { theme } = useTheme();
  const isWeb = Platform.OS === "web";
  const { width } = useWindowDimensions();
  const isDesktop = isWeb && width >= 768;
  const isTop = position.includes("top");
  const visibleToasts = toasts.slice(-4);
  const prevToastIds = useRef<string[]>(visibleToasts.map((t) => t.id));
  const allKnownToastIds = useRef<Set<string>>(
    new Set(toasts.map((t) => t.id)),
  );
  const [isHovered, setIsHovered] = useState(false);

  useEffect(() => {
    return toastManager.subscribeHover(setIsHovered);
  }, []);

  useEffect(() => {
    const currentIds = visibleToasts.map((t) => t.id);
    const hasNewToast = currentIds.some(
      (id) => !allKnownToastIds.current.has(id),
    );

    if (hasNewToast && isHovered) {
      // Brand new toast arrived while expanded - collapse momentarily
      toastManager.setHovered(false);
      setTimeout(() => {
        toastManager.setHovered(true);
      }, 700);
    } else if (isHovered) {
      toastManager.setHovered(true);
      setTimeout(() => {
        toastManager.setHovered(true);
      }, 700);
    }

    // Update known toast IDs
    toasts.forEach((t) => allKnownToastIds.current.add(t.id));
    prevToastIds.current = currentIds;
  }, [visibleToasts, isHovered, toasts]);

  const setHovered = (value: boolean) => toastManager.setHovered(value);

  const pan = Gesture.Pan()
    .onUpdate((event) => {
      const velocity = isTop ? -event.velocityY : event.velocityY;
      if (velocity > 500) {
        runOnJS(setHovered)(true);
      } else if (velocity < -500) {
        runOnJS(setHovered)(false);
      }
    })
    .onEnd((event) => {
      const translationY = isTop ? -event.translationY : event.translationY;
      if (translationY > 50) {
        runOnJS(setHovered)(true);
      } else if (translationY < -50) {
        runOnJS(setHovered)(false);
      }
    });

  const gesture =
    Platform.OS === "web"
      ? Gesture.Hover()
          .onStart(() => {
            runOnJS(setHovered)(true);
          })
          .onEnd(() => {
            runOnJS(setHovered)(false);
          })
      : pan;

  const getPositionStyle = (): ViewStyle => {
    const resolvedPosition =
      position === "auto"
        ? isDesktop
          ? "bottom-right"
          : "top-center"
        : position;

    const styles: ViewStyle = {
      position: "absolute",
      zIndex: 1000,
      paddingHorizontal: isDesktop ? 0 : theme.spacing[4],
    };

    // Set width/maxWidth
    if (isDesktop) {
      styles.width = 400;
    } else {
      styles.width = "100%";
      styles.maxWidth = 400;
    }

    if (resolvedPosition.includes("top")) {
      styles.top = insets.top + theme.spacing[4];
    }
    if (resolvedPosition.includes("bottom")) {
      styles.bottom = insets.bottom + theme.spacing[4];
    }
    if (resolvedPosition.includes("left")) {
      styles.left = insets.left + theme.spacing[4];
      styles.alignItems = "flex-start";
    }
    if (resolvedPosition.includes("right")) {
      styles.right = insets.right + theme.spacing[4];
      styles.alignItems = "flex-end";
    }
    if (resolvedPosition.includes("center")) {
      styles.alignItems = "center";
      // Center the container itself when it has maxWidth
      if (!isDesktop) {
        styles.left = "50%";
        styles.transform = [{ translateX: "-50%" }];
      } else {
        styles.left = 0;
        styles.right = 0;
      }
    }

    return styles;
  };

  return (
    <Portal name="toasties">
      <GestureDetector gesture={gesture}>
        <View style={getPositionStyle()}>
          {visibleToasts.reverse().map((toastState, index) => (
            <Toast
              key={toastState.id}
              {...toastState}
              onOpenChange={(open) => {
                if (!open) toastManager.hide(toastState.id);
              }}
              index={index}
              isLatest={index === 0}
              position={
                position === "auto"
                  ? isDesktop
                    ? "bottom-right"
                    : "top-center"
                  : position
              }
            />
          ))}
        </View>
      </GestureDetector>
    </Portal>
  );
}

export function AndMore({ more }: { more: number }) {
  const { theme } = useTheme();
  return (
    <View
      style={{
        padding: theme.spacing[2],
        paddingHorizontal: theme.spacing[4],
        backgroundColor: theme.colors.muted,
        borderRadius: theme.borderRadius.xl,
        marginTop: theme.spacing[2],
        alignSelf: "center",
      }}
    >
      <Text size="sm" style={{ color: theme.colors.mutedForeground }}>
        and {more} more notification
        {more === 1 ? "" : "s"}
      </Text>
    </View>
  );
}

type PartialBy<T, K extends keyof T> = Omit<T, K> & Partial<Pick<T, K>>;

type ToastProps = PartialBy<Omit<ToastState, "id" | "open">, "position"> & {
  id?: string;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  index?: number;
  isLatest?: boolean;
};

const AnimatedCircle = Animated.createAnimatedComponent(Circle);

function CloseButton({
  onPress,
  isLatest,
  duration,
  animatedCircleProps,
  theme,
  RADIUS,
  CIRCUMFERENCE,
}: {
  onPress: () => void;
  isLatest: boolean;
  duration: number;
  animatedCircleProps: any;
  theme: any;
  RADIUS: number;
  CIRCUMFERENCE: number;
}) {
  return (
    <Pressable
      onPress={onPress}
      style={{
        alignItems: "center",
        justifyContent: "center",
      }}
    >
      <Svg
        width={RADIUS * 2}
        height={RADIUS * 2}
        viewBox={`0 0 ${RADIUS * 2 + 2} ${RADIUS * 2 + 2}`}
      >
        <AnimatedCircle
          stroke={theme.colors.border}
          fill="transparent"
          strokeWidth="2"
          r={RADIUS}
          cx={RADIUS + 1}
          cy={RADIUS + 1}
        />
        {isLatest && duration > 0 && (
          <AnimatedCircle
            animatedProps={animatedCircleProps}
            stroke={theme.colors.primary}
            fill="transparent"
            strokeWidth="2"
            strokeDasharray={CIRCUMFERENCE}
            r={RADIUS}
            cx={RADIUS + 1}
            cy={RADIUS + 1}
            rotation="-90"
            originX={RADIUS + 1}
            originY={RADIUS + 1}
            strokeLinecap="round"
          />
        )}
      </Svg>
      <View style={{ position: "absolute" }}>
        <X color={theme.colors.foreground} size={12} />
      </View>
    </Pressable>
  );
}

export function Toast({
  open = false,
  onOpenChange = () => {},
  title = "",
  description,
  actionLabel = "Action",
  onAction,
  duration = 60,
  index = 0,
  isLatest = true,
  variant = "default",
  render,
  position = "auto",
  id,
  iconLeft: IconLeft,
  iconRight: IconRight,
  onToastPress,
}: ToastProps) {
  const [isHovered, setIsHovered] = useState(false);
  const progress = useSharedValue(1);
  const remainingTime = useSharedValue(duration * 1000);
  const wasOpen = useRef(open);
  const [measuredHeight, setMeasuredHeight] = useState(100);
  const { theme } = useTheme();
  const isWeb = Platform.OS === "web";
  const { width } = useWindowDimensions();
  const isDesktop = isWeb && width >= 768;

  const { top, bottom } = useSafeAreaInsets();

  const isTop = position.includes("top");

  const RADIUS = 12;
  const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

  const dismissTranslateY = useSharedValue(0);
  const dismissTranslateX = useSharedValue(0);

  // Auto-select iconLeft based on variant if not provided
  const defaultIconLeft =
    !IconLeft && !onAction
      ? variant === "success"
        ? CheckCircle
        : variant === "error"
          ? XCircle
          : variant === "info"
            ? Info
            : null
      : null;
  const FinalIconLeft = IconLeft || defaultIconLeft;
  const FinalIconRight = IconRight;

  const animatedCircleProps = useAnimatedProps(() => {
    return {
      strokeDashoffset: CIRCUMFERENCE * (1 - progress.value),
    };
  });

  const opacity = useSharedValue(0);
  const translateY = useSharedValue(isTop ? -100 : 100);
  const scale = useSharedValue(1 - index * 0.05);

  useEffect(() => {
    return toastManager.subscribeHover(setIsHovered);
  }, []);

  useEffect(() => {
    if (open && !wasOpen.current) {
      // Toast just opened
      progress.value = 1;
      remainingTime.value = duration * 1000;
    }
    wasOpen.current = open;

    if (!open) {
      // Close animation
      opacity.value = withTiming(0, { duration: 250 });
      translateY.value = withTiming(isTop ? -100 : 100, { duration: 250 });
      return;
    }

    if (isHovered) {
      // Stack vertically with proper spacing when hovered
      // Calculate cumulative height of all toasts before this one
      let cumulativeHeight = 0;
      const allToasts = toastManager.getToasts().slice(-4).reverse(); // Get visible toasts in same order as rendered
      for (let i = 0; i < index; i++) {
        if (allToasts[i]) {
          cumulativeHeight += toastManager.getToastHeight(allToasts[i].id) + 8; // height + gap
        }
      }

      translateY.value = withTiming(
        isTop ? cumulativeHeight : -cumulativeHeight,
        {
          duration: 750,
          easing: Easing.out(Easing.exp),
        },
      );
      scale.value = withTiming(1, {
        duration: 750,
        easing: Easing.out(Easing.exp),
      });
      opacity.value = withTiming(1, {
        duration: 750,
        easing: Easing.out(Easing.exp),
      });
    } else {
      // Compact stacked view when not hovered
      translateY.value = withTiming((isTop ? index : -index) * 15, {
        duration: 750,
        easing: Easing.out(Easing.exp),
      });
      scale.value = withTiming(1 - index * 0.1, {
        duration: 750,
        easing: Easing.out(Easing.exp),
      });
      opacity.value = withTiming(isLatest ? 1 : 0.95, {
        duration: 750,
        easing: Easing.out(Easing.exp),
      });
    }
  }, [open, isHovered, index, isLatest, measuredHeight, duration, id]);

  useEffect(() => {
    if (open && isLatest && duration > 0) {
      if (isHovered) {
        cancelAnimation(progress);
        remainingTime.value = progress.value * duration * 1000;
      } else {
        progress.value = withTiming(
          0,
          {
            duration: remainingTime.value,
            easing: Easing.linear,
          },
          (finished) => {
            if (finished) {
              runOnJS(onOpenChange)(false);
            }
          },
        );
      }
    } else {
      cancelAnimation(progress);
    }

    return () => {
      cancelAnimation(progress);
    };
  }, [open, isLatest, isHovered, duration]);

  const dismissGestureVertical = Gesture.Pan()
    .enabled(isLatest && !isHovered)
    .activeOffsetY(isTop ? [-10, 999999] : [-999999, 10])
    .onUpdate((event) => {
      const direction = isTop ? -1 : 1;
      const movement = event.translationY * direction;
      if (movement > 0) {
        dismissTranslateY.value = event.translationY;
        dismissTranslateX.value = 0;
      }
    })
    .onEnd((event) => {
      const direction = isTop ? -1 : 1;
      const movement = event.translationY * direction;
      const velocity = event.velocityY * direction;

      if (movement > 50 || velocity > 500) {
        // Dismiss - slide out in expansion direction
        dismissTranslateY.value = withTiming(
          isTop ? -200 : 200,
          { duration: 250 },
          (finished) => {
            if (finished) {
              runOnJS(onOpenChange)(false);
            }
          },
        );
        opacity.value = withTiming(0, { duration: 250 });
      } else {
        // Spring back
        dismissTranslateY.value = withTiming(0, { duration: 200 });
      }
    });

  const dismissGestureHorizontal = Gesture.Pan()
    .enabled(isLatest || isHovered)
    .activeOffsetX([-10, 10])
    .onUpdate((event) => {
      if (event.translationX < 0) {
        dismissTranslateX.value = event.translationX;
        dismissTranslateY.value = 0;
      }
    })
    .onEnd((event) => {
      // Check for horizontal swipe left
      if (event.translationX < -100 || event.velocityX < -500) {
        dismissTranslateX.value = withTiming(
          -400,
          { duration: 250 },
          (finished) => {
            if (finished) {
              runOnJS(onOpenChange)(false);
            }
          },
        );
        opacity.value = withTiming(0, { duration: 250 });
      } else {
        // Spring back
        dismissTranslateX.value = withTiming(0, { duration: 200 });
      }
    });

  const dismissGesture = Gesture.Race(
    dismissGestureVertical,
    dismissGestureHorizontal,
  );

  const animatedStyle = useAnimatedStyle(() => {
    return {
      opacity: opacity.value,
      transform: [
        // +22 is to get it just below the header
        {
          translateY:
            translateY.value +
            dismissTranslateY.value +
            (isTop ? top / 2 : -bottom),
        },
        { translateX: dismissTranslateX.value },
        { scale: scale.value },
      ],
      zIndex: 1000 - index,
    };
  });

  const variantStyles = {
    default: {
      backgroundColor: theme.colors.secondary,
      borderColor: theme.colors.border,
    },
    success: {
      backgroundColor: theme.colors.success,
      borderColor: theme.colors.success,
    },
    error: {
      backgroundColor: theme.colors.destructive,
      borderColor: theme.colors.destructive,
    },
    info: {
      backgroundColor: theme.colors.info,
      borderColor: theme.colors.info,
    },
  };

  const buttonTypeMap = {
    default: "primary",
    success: "success",
    error: "primary",
    info: "secondary",
  } as const;

  return (
    <Animated.View
      onLayout={(l) => {
        const height = l.nativeEvent.layout.height;
        setMeasuredHeight(height);
        if (id) {
          toastManager.updateToastHeight(id, height);
        }
      }}
      style={[
        isTop ? styles.containerTop : styles.containerBottom,
        animatedStyle,
      ]}
    >
      <GestureDetector gesture={dismissGesture}>
        <Pressable
          onPress={onToastPress}
          disabled={!onToastPress}
          style={[
            styles.toast,
            {
              borderRadius: theme.borderRadius.xl,
              flexDirection: "column",
              justifyContent: "space-between",
              alignItems: "center",
              padding: theme.spacing[4],
              width: "100%",
            },
            variantStyles[variant],
          ]}
        >
          {render ? (
            render({ close: () => onOpenChange(false), action: onAction })
          ) : (
            <>
              <View
                style={{
                  flexDirection: "row",
                  alignItems: "flex-start",
                  justifyContent: "space-between",
                  width: "100%",
                  gap: theme.spacing[4],
                }}
              >
                <View
                  style={{
                    flexDirection: "row",
                    flex: 1,
                    gap: theme.spacing[3],
                  }}
                >
                  {FinalIconLeft && (
                    <View style={{ paddingTop: 2 }}>
                      <FinalIconLeft color={theme.colors.foreground} />
                    </View>
                  )}
                  <View style={{ flex: 1 }}>
                    <Text size="lg">{title}</Text>
                    {description ? <Text>{description}</Text> : null}
                  </View>
                </View>
                {FinalIconRight && !onAction ? (
                  <View
                    style={{
                      gap: theme.spacing[2],
                      height: "100%",
                      justifyContent: "space-between",
                    }}
                  >
                    <Pressable onPress={onToastPress}>
                      <FinalIconRight color={theme.colors.foreground} />
                    </Pressable>
                    <CloseButton
                      onPress={() => onOpenChange(false)}
                      isLatest={isLatest}
                      duration={duration}
                      animatedCircleProps={animatedCircleProps}
                      theme={theme}
                      RADIUS={RADIUS}
                      CIRCUMFERENCE={CIRCUMFERENCE}
                    />
                  </View>
                ) : !onAction ? (
                  <CloseButton
                    onPress={() => onOpenChange(false)}
                    isLatest={isLatest}
                    duration={duration}
                    animatedCircleProps={animatedCircleProps}
                    theme={theme}
                    RADIUS={RADIUS}
                    CIRCUMFERENCE={CIRCUMFERENCE}
                  />
                ) : null}
              </View>
              {onAction && (
                <View
                  style={{
                    gap: theme.spacing[1],
                    flexDirection: "row",
                    justifyContent: "flex-end",
                    width: "100%",
                  }}
                >
                  <Button
                    width="min"
                    variant={buttonTypeMap[variant]}
                    onPress={onAction}
                  >
                    <Text style={{ color: theme.colors.foreground }}>
                      {actionLabel}
                    </Text>
                  </Button>
                  <Button
                    variant="secondary"
                    width="min"
                    onPress={() => onOpenChange(false)}
                  >
                    <Text style={{ color: theme.colors.foreground }}>
                      Close
                    </Text>
                  </Button>
                </View>
              )}
            </>
          )}
        </Pressable>
      </GestureDetector>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  providerContainer: {
    position: "absolute",
    zIndex: 1000,
  },
  containerBottom: {
    position: "absolute",
    bottom: 0,
    left: 0,
    right: 0,
    alignItems: "center",
  },
  containerTop: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    alignItems: "center",
  },
  toast: {
    opacity: 0.99,
    borderWidth: 1,
    gap: 8,
  },
  button: {
    borderWidth: 1,
    borderRadius: 8,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: "transparent",
  },
});
