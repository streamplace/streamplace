import BottomSheet, { BottomSheetScrollView } from "@gorhom/bottom-sheet";
import * as DropdownMenuPrimitive from "@rn-primitives/dropdown-menu";
import {
  Check,
  CheckCircle,
  ChevronLeft,
  ChevronRight,
  Circle,
} from "lucide-react-native";
import React, {
  createContext,
  forwardRef,
  ReactNode,
  startTransition,
  useContext,
  useRef,
  useState,
} from "react";
import { Pressable, StyleSheet, useWindowDimensions, View } from "react-native";
import Animated, {
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import {
  a,
  borderRadius,
  fontSize,
  gap,
  layout,
  ml,
  p,
  pb,
  pl,
  pr,
  pt,
  px,
  py,
  right,
} from "../../lib/theme/atoms";
import { useTheme } from "../../ui";
import {
  objectFromObjects,
  TextContext as TextClassContext,
} from "./primitives/text";
import { Text } from "./text";

// Navigation stack context for bottom sheet menus
interface NavigationStackItem {
  key: string;
  title?: string;
  content: ReactNode | ((state: { pressed: boolean }) => ReactNode);
}

interface NavigationStackContextValue {
  stack: NavigationStackItem[];
  push: (item: NavigationStackItem) => void;
  pop: () => void;
  isNested: boolean;
}

const NavigationStackContext =
  createContext<NavigationStackContextValue | null>(null);

const useNavigationStack = () => {
  const context = useContext(NavigationStackContext);
  return context;
};

// Context to capture submenu content for mobile navigation
interface SubMenuContextValue {
  title?: string;
  renderContent: () => ReactNode;
  setRenderContent: (renderer: () => ReactNode) => void;
  setTitle: (title: string) => void;
  trigger: () => void;
  key: string | null;
}

const SubMenuContext = createContext<SubMenuContextValue | null>(null);

// Context for RadioGroup on native
interface RadioGroupContextValue {
  value?: string;
  onValueChange?: (value: string) => void;
}

const RadioGroupContext = createContext<RadioGroupContextValue | null>(null);

export const DropdownMenu = DropdownMenuPrimitive.Root;
export const DropdownMenuTrigger = DropdownMenuPrimitive.Trigger;
export const DropdownMenuPortal = DropdownMenuPrimitive.Portal;

export const DropdownMenuRadioGroup = forwardRef<
  React.ElementRef<typeof DropdownMenuPrimitive.RadioGroup>,
  React.ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.RadioGroup>
>(({ children, value, onValueChange, ...props }, ref) => {
  return (
    <RadioGroupContext.Provider value={{ value, onValueChange }}>
      {children}
    </RadioGroupContext.Provider>
  );
});

export const DropdownMenuSub = forwardRef<any, any>(
  ({ children, ...props }, ref) => {
    const navStack = useNavigationStack();
    const [subMenuTitle, setSubMenuTitle] = useState<string | undefined>();
    const renderContentRef = useRef<(() => ReactNode) | null>(null);
    const [subMenuKey, setSubMenuKey] = useState<string | null>(null);

    const trigger = () => {
      if (renderContentRef.current && navStack) {
        const key = `submenu-${Date.now()}`;
        setSubMenuKey(key);
        navStack.push({
          key,
          title: subMenuTitle,
          content: (props: any) => {
            const renderFn = renderContentRef.current;
            return renderFn ? renderFn() : null;
          },
        });
      }
    };

    const setRenderContent = (renderer: () => ReactNode) => {
      renderContentRef.current = renderer;
    };

    const contextValue = React.useMemo(
      () => ({
        renderContent: () => renderContentRef.current?.(),
        setRenderContent,
        title: subMenuTitle,
        setTitle: setSubMenuTitle,
        trigger,
        key: subMenuKey,
      }),
      [subMenuTitle, subMenuKey, navStack],
    );

    return (
      <SubMenuContext.Provider value={contextValue}>
        {children}
      </SubMenuContext.Provider>
    );
  },
);

export const DropdownMenuBottomSheet = forwardRef<
  any,
  DropdownMenuPrimitive.ContentProps & {
    overlayStyle?: any;
    portalHost?: string;
  }
>(function DropdownMenuBottomSheet(
  { overlayStyle, portalHost, children, ...rest },
  _ref,
) {
  const { onOpenChange } = DropdownMenuPrimitive.useRootContext();
  const { zero: zt, theme } = useTheme();
  const sheetRef = useRef<BottomSheet>(null);
  const { width } = useWindowDimensions();
  const isWide = width >= 450;
  const sheetWidth = isWide ? 450 : width;
  const horizontalMargin = isWide ? (width - sheetWidth) / 2 : 0;

  const insets = useSafeAreaInsets();

  const [stack, setStack] = useState<NavigationStackItem[]>([
    { key: "root", content: children },
  ]);

  React.useEffect(() => {
    setStack((prev) => {
      if (!Array.isArray(prev) || prev.length === 0) {
        return [{ key: "root", content: children }];
      }
      const newStack = [...prev];
      newStack[0] = { ...newStack[0], content: children };
      return newStack;
    });
  }, [children]);

  const slideAnim = useSharedValue(0);
  const fadeAnim = useSharedValue(1);

  const push = (item: NavigationStackItem) => {
    setStack((prev) => {
      if (!Array.isArray(prev))
        return [{ key: "root", content: children }, item];
      return [...prev, item];
    });

    slideAnim.value = 40;
    fadeAnim.value = 0;
    slideAnim.value = withTiming(0, { duration: 350 });
    fadeAnim.value = withTiming(1, { duration: 350 });
  };

  const popStack = () => {
    startTransition(() => {
      setStack((prev) => {
        if (!Array.isArray(prev) || prev.length <= 1) {
          return [{ key: "root", content: children }];
        }
        return prev.slice(0, -1);
      });
    });
  };

  const resetAnimationValues = () => {
    setTimeout(() => {
      slideAnim.value = 0;
      fadeAnim.value = 1;
    }, 5);
  };

  const pop = () => {
    if (stack.length <= 1) return;

    slideAnim.value = withTiming(40, { duration: 150 });
    fadeAnim.value = withTiming(0, { duration: 150 }, (finished) => {
      if (finished) {
        runOnJS(popStack)();
        runOnJS(resetAnimationValues)();
      }
    });
  };

  const animatedStyle = useAnimatedStyle(() => ({
    transform: [{ translateX: slideAnim.value }],
    opacity: fadeAnim.value,
  }));

  const headerAnimatedStyle = useAnimatedStyle(() => ({
    opacity: fadeAnim.value,
  }));

  const currentLevel = stack[stack.length - 1];
  const isNested = stack.length > 1;

  const onBackgroundTap = () => {
    if (sheetRef.current) sheetRef.current?.close();

    setTimeout(() => {
      onOpenChange?.(false);
    }, 300);
  };

  if (!currentLevel) {
    return null;
  }

  return (
    <DropdownMenuPrimitive.Portal hostName={portalHost}>
      <NavigationStackContext.Provider value={{ stack, push, pop, isNested }}>
        <BottomSheet
          ref={sheetRef}
          enablePanDownToClose
          enableDynamicSizing
          detached={isWide}
          bottomInset={isWide ? 0 : 0}
          backdropComponent={({ style }) => (
            <Pressable
              style={[style, StyleSheet.absoluteFill]}
              onPress={() => onBackgroundTap()}
            />
          )}
          onClose={() => onOpenChange?.(false)}
          style={[
            overlayStyle,
            StyleSheet.flatten(rest.style),
            isWide && { marginHorizontal: horizontalMargin },
          ]}
          backgroundStyle={[zt.bg.popover, a.radius.all.md, a.shadows.md, p[1]]}
          handleIndicatorStyle={[
            a.sizes.width[12],
            a.sizes.height[1],
            zt.bg.mutedForeground,
          ]}
        >
          {isNested && (
            <Animated.View
              style={[
                headerAnimatedStyle,
                a.layout.flex.row,
                a.layout.flex.alignCenter,
                px[4],
                pb[2],
                {
                  borderBottomWidth: 1,
                  borderBottomColor: theme.colors.border,
                },
              ]}
            >
              <Pressable
                onPress={pop}
                style={[
                  a.layout.flex.row,
                  a.layout.flex.alignCenter,
                  gap.all[2],
                ]}
                hitSlop={80}
              >
                <ChevronLeft size={20} color={theme.colors.foreground} />
                {currentLevel?.title ? (
                  <Text size="lg">{currentLevel.title}</Text>
                ) : null}
              </Pressable>
            </Animated.View>
          )}
          <Animated.View style={animatedStyle}>
            <BottomSheetScrollView
              style={[px[4]]}
              contentContainerStyle={{
                paddingBottom: insets.bottom + 50,
                overflow: "hidden",
              }}
            >
              {stack.map((level, index) => {
                const isCurrent = index === stack.length - 1;
                return (
                  <View
                    key={level.key}
                    style={[{ display: isCurrent ? "flex" : "none" }]}
                  >
                    {typeof level.content === "function"
                      ? level.content({ pressed: true })
                      : level.content}
                  </View>
                );
              })}
            </BottomSheetScrollView>
          </Animated.View>
        </BottomSheet>
      </NavigationStackContext.Provider>
    </DropdownMenuPrimitive.Portal>
  );
});

export const DropdownMenuSubTrigger = forwardRef<
  any,
  DropdownMenuPrimitive.SubTriggerProps & {
    inset?: boolean;
    subMenuTitle?: string;
  } & {
    ref?: React.RefObject<DropdownMenuPrimitive.SubTriggerRef>;
    className?: string;
    inset?: boolean;
    children?: React.ReactNode;
  }
>(({ inset, children, subMenuTitle, ...props }, ref) => {
  const subMenuContext = useContext(SubMenuContext);
  const { icons } = useTheme();

  React.useEffect(() => {
    if (subMenuContext && subMenuTitle) {
      subMenuContext.setTitle(subMenuTitle);
    }
  }, [subMenuContext, subMenuTitle]);

  return (
    <Pressable
      onPress={() => {
        subMenuContext?.trigger();
      }}
      {...props}
    >
      <View
        style={[
          inset && gap[2],
          layout.flex.row,
          layout.flex.alignCenter,
          a.radius.all.sm,
          py[1],
          pl[2],
          pr[2],
        ]}
      >
        {typeof children === "function" ? (
          children({ pressed: true })
        ) : typeof children === "string" ? (
          <Text>{children}</Text>
        ) : (
          children
        )}
        <View style={[a.layout.position.absolute, a.position.right[1]]}>
          <ChevronRight size={18} color={icons.color.muted} />
        </View>
      </View>
    </Pressable>
  );
});

export const DropdownMenuSubContent = forwardRef<
  any,
  DropdownMenuPrimitive.SubContentProps & { children?: ReactNode }
>(({ children, ...props }, ref) => {
  const subMenuContext = useContext(SubMenuContext);
  const navStack = useNavigationStack();
  const prevChildrenRef = useRef<ReactNode>(null);

  React.useEffect(() => {
    if (subMenuContext && navStack) {
      if (prevChildrenRef.current === children) {
        return;
      }

      prevChildrenRef.current = children;
      subMenuContext.setRenderContent(() => children);
    }
  }, [children, subMenuContext, navStack]);

  return null;
});

export const DropdownMenuContent = forwardRef<
  any,
  DropdownMenuPrimitive.ContentProps & {
    overlayStyle?: any;
    portalHost?: string;
  }
>(({ overlayStyle, portalHost, style, children, ...props }, ref) => {
  return (
    <DropdownMenuBottomSheet {...props}>{children}</DropdownMenuBottomSheet>
  );
});

export const DropdownMenuContentWithoutPortal = forwardRef<
  any,
  DropdownMenuPrimitive.ContentProps & {
    overlayStyle?: any;
    maxHeightPercentage?: number;
  }
>(
  (
    { overlayStyle, maxHeightPercentage = 0.8, children, style, ...props },
    ref,
  ) => {
    return (
      <DropdownMenuBottomSheet {...props}>{children}</DropdownMenuBottomSheet>
    );
  },
);

export const ResponsiveDropdownMenuContent = forwardRef<
  any,
  any & { onModeChange?: (isSheet: boolean) => void }
>(({ children, onModeChange, ...props }, ref) => {
  React.useEffect(() => {
    onModeChange?.(true);
  }, [onModeChange]);

  return (
    <DropdownMenuBottomSheet ref={ref} {...props}>
      {children}
    </DropdownMenuBottomSheet>
  );
});

export const DropdownMenuItem = forwardRef<
  any,
  DropdownMenuPrimitive.ItemProps & { inset?: boolean; disabled?: boolean }
>(({ inset, disabled, style, children, ...props }, ref) => {
  const { theme } = useTheme();
  return (
    <Pressable {...props}>
      <TextClassContext.Provider
        value={objectFromObjects([
          { color: theme.colors.popoverForeground },
          a.fontSize.base,
        ])}
      >
        <View
          style={[
            a.layout.flex.row,
            a.layout.flex.alignCenter,
            a.radius.all.sm,
            py[1],
            pl[2],
            pr[2],
          ]}
        >
          {typeof children === "function" ? (
            children({ pressed: true })
          ) : typeof children === "string" ? (
            <Text style={[inset && gap[2], disabled && { opacity: 0.5 }]}>
              {children}
            </Text>
          ) : (
            children
          )}
        </View>
      </TextClassContext.Provider>
    </Pressable>
  );
});

export const DropdownMenuCheckboxItem = forwardRef<
  any,
  DropdownMenuPrimitive.CheckboxItemProps & {
    ref?: React.RefObject<DropdownMenuPrimitive.CheckboxItemRef>;
    children?: React.ReactNode;
  }
>(({ children, checked, ...props }, ref) => {
  const { theme } = useTheme();

  return (
    <Pressable
      onPress={(e) => {
        props.onCheckedChange?.(!checked);
        props.onPress?.(e);
      }}
      {...props}
    >
      <View
        style={[
          a.layout.flex.row,
          a.layout.flex.alignCenter,
          a.radius.all.sm,
          py[1],
          pl[2],
          pr[2],
          pr[8],
        ]}
      >
        {children}
        <View style={[pl[1], layout.position.absolute, right[1]]}>
          {checked ? (
            <CheckCircle
              size={14}
              strokeWidth={3}
              color={theme.colors.foreground}
            />
          ) : (
            <Circle
              size={14}
              strokeWidth={3}
              color={theme.colors.mutedForeground}
            />
          )}
        </View>
      </View>
    </Pressable>
  );
});

export const DropdownMenuRadioItem = forwardRef<
  any,
  DropdownMenuPrimitive.RadioItemProps & {
    ref?: React.RefObject<DropdownMenuPrimitive.RadioItemRef>;
    children?: React.ReactNode;
    value?: string;
  }
>(({ children, value, ...props }, ref) => {
  const { theme } = useTheme();
  const radioGroupContext = useContext(RadioGroupContext);
  const isSelected = radioGroupContext?.value === value;

  return (
    <Pressable
      onPress={(e) => {
        if (value && radioGroupContext?.onValueChange) {
          radioGroupContext.onValueChange(value);
        }
        props.onPress?.(e);
      }}
      {...props}
    >
      <View
        style={[
          a.layout.flex.row,
          a.layout.flex.alignCenter,
          a.radius.all.sm,
          py[1],
          pl[2],
          pr[8],
        ]}
      >
        {children}
        {isSelected && (
          <View style={[pl[1], layout.position.absolute, right[1]]}>
            <Check size={14} strokeWidth={3} color={theme.colors.foreground} />
          </View>
        )}
      </View>
    </Pressable>
  );
});

export const DropdownMenuLabel = forwardRef<
  any,
  DropdownMenuPrimitive.LabelProps & { inset?: boolean }
>(({ inset, ...props }, ref) => {
  const { theme } = useTheme();
  return (
    <Text
      ref={ref}
      style={
        [
          px[2],
          py[2],
          { color: theme.colors.textMuted },
          a.fontSize.base,
          (inset && gap[2]) as any,
        ] as any
      }
      {...props}
    />
  );
});

export const DropdownMenuSeparator = forwardRef<
  any,
  DropdownMenuPrimitive.SeparatorProps
>((props, ref) => {
  const { theme } = useTheme();
  return (
    <View
      ref={ref}
      style={[
        {
          borderBottomWidth: 1,
          borderBottomColor: theme.colors.border,
          marginVertical: -0.5,
        },
      ]}
      {...props}
    />
  );
});

export function DropdownMenuShortcut(props: any) {
  const { theme } = useTheme();
  return (
    <Text
      style={[
        ml.auto,
        { color: theme.colors.textMuted },
        a.fontSize.sm,
        a.letterSpacing.widest,
      ]}
      {...props}
    />
  );
}

export const DropdownMenuGroup = forwardRef<
  any,
  { inset?: boolean; title?: string; children: ReactNode }
>((props, ref) => {
  const { theme } = useTheme();
  const { inset, title, children, ...rest } = props;
  return (
    <View style={[pt[2], inset && gap[2]]} ref={ref} {...rest}>
      {title && (
        <Text style={[{ color: theme.colors.textMuted }, pb[1], pl[2]]}>
          {title}
        </Text>
      )}
      <View
        style={[
          { backgroundColor: theme.colors.muted },
          p[2],
          gap.all[1],
          { borderRadius: borderRadius.lg },
        ]}
      >
        {children}
      </View>
    </View>
  );
});

export const DropdownMenuInfo = forwardRef<any, any>(
  ({ description, ...props }, ref) => {
    const { theme } = useTheme();
    return (
      <Text
        style={[
          { color: theme.colors.textMuted },
          pt[1],
          pl[2],
          pb[2],
          fontSize.sm,
        ]}
      >
        {description}
      </Text>
    );
  },
);
