import {
  Children,
  cloneElement,
  forwardRef,
  isValidElement,
  ReactNode,
} from "react";
import { Animated, Platform, View, ViewStyle } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import {
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";
import {
  a,
  borderRadius,
  fontSize,
  gap,
  mt,
  mx,
  p,
  pb,
  pl,
  pr,
  pt,
  px,
  py,
} from "../../lib/theme/atoms";
import { mergeStyles, useTheme } from "../../ui";
import { Text } from "./text";

export interface MenuContainerProps {
  children: ReactNode;
  style?: ViewStyle;
}

export const MenuContainer = forwardRef<View, MenuContainerProps>(
  ({ children, style }, ref) => {
    const { theme } = useTheme();
    return (
      <View ref={ref} style={[gap.all[4], mt[4], mx[2], style]}>
        {children}
      </View>
    );
  },
);

export interface MenuGroupProps {
  children: ReactNode;
  style?: ViewStyle;
}

export const MenuGroup = forwardRef<View, MenuGroupProps>(
  ({ children, style }, ref) => {
    const { theme } = useTheme();
    return (
      <View
        ref={ref}
        style={[
          { backgroundColor: theme.colors.muted + "c0" },
          Platform.OS === "web" ? [px[1], py[1]] : p[1],
          gap.all[1],
          { borderRadius: borderRadius.lg },
          style,
        ]}
      >
        {children}
      </View>
    );
  },
);

export interface MenuItemProps {
  children: ReactNode;
  disabled?: boolean;
  style?: ViewStyle;
  onPress?: () => void;
  draggable?: boolean;
  dragHandle?: ReactNode;
  _dragIndex?: number;
  _dragTotalItems?: number;
  _onDragMove?: (fromIndex: number, toIndex: number) => void;
  _onDragEnd?: (fromIndex: number, toIndex: number) => void;
}

export const MenuItem = forwardRef<View, MenuItemProps>(
  (
    {
      children,
      disabled,
      style,
      draggable,
      dragHandle,
      _dragIndex,
      _dragTotalItems,
      _onDragMove,
      _onDragEnd,
    },
    ref,
  ) => {
    const { theme } = useTheme();

    if (
      draggable &&
      _dragIndex !== undefined &&
      _dragTotalItems !== undefined &&
      _onDragMove &&
      _onDragEnd
    ) {
      const translateY = useSharedValue(0);
      const isDragging = useSharedValue(false);
      const ITEM_HEIGHT = 60;

      const panGesture = Gesture.Pan()
        .onStart(() => {
          isDragging.value = true;
        })
        .onUpdate((event) => {
          translateY.value = event.translationY;

          const newIndex = Math.round(
            _dragIndex + translateY.value / ITEM_HEIGHT,
          );
          const clampedIndex = Math.max(
            0,
            Math.min(_dragTotalItems - 1, newIndex),
          );

          if (clampedIndex !== _dragIndex) {
            runOnJS(_onDragMove)(_dragIndex, clampedIndex);
          }
        })
        .onEnd(() => {
          const newIndex = Math.round(
            _dragIndex + translateY.value / ITEM_HEIGHT,
          );
          const clampedIndex = Math.max(
            0,
            Math.min(_dragTotalItems - 1, newIndex),
          );

          runOnJS(_onDragEnd)(_dragIndex, clampedIndex);

          translateY.value = withSpring(0);
          isDragging.value = false;
        });

      const animatedStyle = useAnimatedStyle(() => ({
        transform: [{ translateY: translateY.value }],
        zIndex: isDragging.value ? 100 : 1,
        opacity: isDragging.value ? 0.8 : 1,
      }));

      return (
        <Animated.View style={animatedStyle}>
          <View
            ref={ref}
            style={[
              a.layout.flex.row,
              a.layout.flex.alignCenter,
              a.radius.all.sm,
              py[1],
              pl[3],
              pr[2],
              disabled && { opacity: 0.5 },
              style,
            ]}
          >
            {dragHandle && (
              <GestureDetector gesture={panGesture}>
                <View style={{ marginRight: 8 }}>{dragHandle}</View>
              </GestureDetector>
            )}
            {typeof children === "string" ? (
              <Text style={{ color: theme.colors.popoverForeground }}>
                {children}
              </Text>
            ) : (
              children
            )}
          </View>
        </Animated.View>
      );
    }

    return (
      <View
        ref={ref}
        style={[
          a.layout.flex.row,
          a.layout.flex.alignCenter,
          a.radius.all.sm,
          py[1],
          pl[3],
          pr[2],
          disabled && { opacity: 0.5 },
          style,
        ]}
      >
        {typeof children === "string" ? (
          <Text style={{ color: theme.colors.popoverForeground }}>
            {children}
          </Text>
        ) : (
          children
        )}
      </View>
    );
  },
);

export interface MenuLabelProps {
  children: ReactNode;
  style?: ViewStyle;
}

export const MenuLabel = forwardRef<View, MenuLabelProps>(
  ({ children, style }, ref) => {
    const { theme } = useTheme();
    return (
      <Text
        ref={ref as any}
        style={mergeStyles(
          px[4],
          py[2],
          { color: theme.colors.textMuted },
          a.fontSize.base,
          style,
        )}
      >
        {children}
      </Text>
    );
  },
);

export interface MenuSeparatorProps {
  style?: ViewStyle;
}

export const MenuSeparator = forwardRef<View, MenuSeparatorProps>(
  ({ style }, ref) => {
    const { theme } = useTheme();
    return (
      <View
        ref={ref}
        style={[
          mx[2],
          {
            height: 1,
            backgroundColor: theme.colors.border,
            marginVertical: 0,
          },
          style,
        ]}
      />
    );
  },
);

export interface MenuInfoProps {
  description: string;
  style?: ViewStyle;
}

export const MenuInfo = forwardRef<View, MenuInfoProps>(
  ({ description, style }, ref) => {
    const { theme } = useTheme();
    return (
      <Text
        ref={ref as any}
        style={mergeStyles(
          { color: theme.colors.textMuted, marginTop: -8 },
          pt[1],
          pl[4],
          pb[2],
          fontSize.sm,
          style,
        )}
      >
        {description}
      </Text>
    );
  },
);

export interface MenuDraggableGroupProps {
  children: ReactNode;
  onMove: (fromIndex: number, toIndex: number) => void;
  onDragEnd: (fromIndex: number, toIndex: number) => void;
  dragHandle?: ReactNode;
  style?: ViewStyle;
}

export const MenuDraggableGroup = forwardRef<View, MenuDraggableGroupProps>(
  ({ children, onMove, onDragEnd, dragHandle, style }, ref) => {
    const { theme } = useTheme();

    const childrenArray = Children.toArray(children);
    const draggableItems = childrenArray.filter(
      (child) =>
        isValidElement(child) &&
        (child.type === MenuItem || child.type === MenuSeparator),
    );

    let itemIndex = 0;
    const enhancedChildren = Children.map(children, (child) => {
      if (isValidElement(child)) {
        if (child.type === MenuItem) {
          const currentIndex = itemIndex;
          itemIndex++;

          return cloneElement(child, {
            draggable: true,
            dragHandle: dragHandle || child.props.dragHandle,
            _dragIndex: currentIndex,
            _dragTotalItems: draggableItems.filter(
              (c) => isValidElement(c) && c.type === MenuItem,
            ).length,
            _onDragMove: onMove,
            _onDragEnd: onDragEnd,
          } as any);
        }
        if (child.type === MenuSeparator) {
          return child;
        }
      }
      return child;
    });

    return (
      <View
        ref={ref}
        style={[
          { backgroundColor: theme.colors.muted + "c0" },
          Platform.OS === "web" ? [px[1], py[1]] : p[1],
          gap.all[1],
          { borderRadius: borderRadius.lg },
          style,
        ]}
      >
        {enhancedChildren}
      </View>
    );
  },
);
