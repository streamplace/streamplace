import * as RadixDropdownMenu from "@radix-ui/react-dropdown-menu";
import * as DropdownMenuPrimitive from "@rn-primitives/dropdown-menu";
import {
  Check,
  ChevronDown,
  ChevronRight,
  ChevronUp,
} from "lucide-react-native";
import React, { forwardRef, ReactNode } from "react";
import {
  Platform,
  ScrollView,
  StyleSheet,
  useWindowDimensions,
  View,
} from "react-native";
import {
  a,
  borderRadius,
  fontSize,
  gap,
  layout,
  ml,
  mt,
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
import Animated, {
  Easing,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";

export const DropdownMenu = DropdownMenuPrimitive.Root;
export const DropdownMenuTrigger = DropdownMenuPrimitive.Trigger;
export const DropdownMenuPortal = DropdownMenuPrimitive.Portal;

export const DropdownMenuRadioGroup = forwardRef<
  React.ElementRef<typeof DropdownMenuPrimitive.RadioGroup>,
  React.ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.RadioGroup>
>(({ children, ...props }, ref) => {
  return (
    <DropdownMenuPrimitive.RadioGroup ref={ref} {...props}>
      {children}
    </DropdownMenuPrimitive.RadioGroup>
  );
});

export const DropdownMenuSub = forwardRef<any, any>(
  ({ children, ...props }, ref) => {
    return (
      <DropdownMenuPrimitive.Sub ref={ref} {...props}>
        {children}
      </DropdownMenuPrimitive.Sub>
    );
  },
);

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
>(({ inset, children, subMenuTitle, style, ...props }, ref) => {
  const { icons } = useTheme();
  const { open } = DropdownMenuPrimitive.useSubContext();
  const Icon =
    Platform.OS === "web" ? ChevronRight : open ? ChevronUp : ChevronDown;

  return (
    <TextClassContext.Provider
      value={objectFromObjects([
        a.textColors.primary[500],
        a.fontSize.base,
        open && a.textColors.primary[700],
      ])}
    >
      <DropdownMenuPrimitive.SubTrigger ref={ref} {...props}>
        <View
          style={[
            inset && gap[2],
            layout.flex.row,
            layout.flex.alignCenter,
            p[2],
            pr[8],
            style,
          ]}
        >
          {children}
          <View style={[a.layout.position.absolute, a.position.right[1]]}>
            <Icon size={18} color={icons.color.muted} />
          </View>
        </View>
      </DropdownMenuPrimitive.SubTrigger>
    </TextClassContext.Provider>
  );
});

export const DropdownMenuSubContent = forwardRef<
  any,
  DropdownMenuPrimitive.SubContentProps & {
    children?: ReactNode;
    portalHost?: string;
    sideOffset?: number;
    alignOffset?: number;
    avoidCollisions?: boolean;
  }
>(
  (
    {
      children,
      portalHost,
      sideOffset,
      alignOffset,
      avoidCollisions = true,
      ...props
    },
    ref,
  ) => {
    const { zero: zt } = useTheme();

    const [portalContainer, setPortalContainer] =
      React.useState<HTMLElement | null>(null);

    React.useEffect(() => {
      if (Platform.OS === "web" && portalHost) {
        const element = document.querySelector<HTMLElement>(
          `[data-portal-host="${portalHost}"]`,
        );
        setPortalContainer(element);
      }
    }, [portalHost]);

    const styles = [
      a.sizes.minWidth[64],
      a.sizes.maxWidth[64],
      a.overflow.hidden,
      a.radius.all.md,
      a.borders.width.thin,
      zt.border.default,
      mt[1],
      zt.bg.popover,
      p[1],
      a.shadows.md,
    ];

    // On web, use Radix directly to support custom portal container
    if (Platform.OS === "web") {
      const { forceMount } = props;
      // Flatten RN style array into a plain CSS object for DOM
      const flattenedStyles = StyleSheet.flatten(styles);
      return (
        <RadixDropdownMenu.Portal
          {...(portalContainer ? { container: portalContainer } : {})}
        >
          <RadixDropdownMenu.SubContent
            ref={ref}
            style={flattenedStyles as React.CSSProperties}
            forceMount={forceMount}
            sideOffset={sideOffset}
            alignOffset={alignOffset}
            avoidCollisions={avoidCollisions}
          >
            {children}
          </RadixDropdownMenu.SubContent>
        </RadixDropdownMenu.Portal>
      );
    }

    // On native, use rn-primitives
    return (
      <DropdownMenuPrimitive.SubContent ref={ref} style={styles} {...props}>
        {children}
      </DropdownMenuPrimitive.SubContent>
    );
  },
);

// The floating menu surface: refined radius + a deep soft shadow, and a
// spring-eased scale/fade/rise on open so it grows out of its trigger instead
// of popping in. Shared by every dropdown in the app.
function AnimatedDropdownPanel({
  children,
  style,
  maxHeight,
}: {
  children: React.ReactNode;
  style?: any;
  maxHeight: number;
}) {
  const { theme } = useTheme();
  const c = theme.colors;
  const progress = useSharedValue(0);
  React.useEffect(() => {
    progress.value = withTiming(1, {
      duration: 190,
      easing: Easing.bezier(0.16, 1, 0.3, 1),
    });
  }, []);
  const animated = useAnimatedStyle(() => ({
    opacity: progress.value,
    transform: [
      { scale: 0.95 + 0.05 * progress.value },
      { translateY: -8 * (1 - progress.value) },
    ],
  }));
  return (
    <Animated.View
      style={[
        {
          maxWidth: 400,
          maxHeight,
          overflow: "hidden",
          borderRadius: 14,
          borderWidth: 1,
          borderColor: c.borderStrong,
          backgroundColor: c.popover,
          ...a.shadows.xl,
          shadowOpacity: 0.5,
          shadowRadius: 30,
          shadowOffset: { width: 0, height: 16 },
          // web-only: grow out of the trigger corner
          transformOrigin: "top right" as any,
        },
        animated,
        style,
      ]}
    >
      {children}
    </Animated.View>
  );
}

export const DropdownMenuContent = forwardRef<
  any,
  DropdownMenuPrimitive.ContentProps & {
    overlayStyle?: any;
    portalHost?: string;
  }
>(({ overlayStyle, portalHost, style, children, ...props }, ref) => {
  const { zero: zt } = useTheme();
  const { height } = useWindowDimensions();
  const maxHeight = height * 0.9;

  const [portalContainer, setPortalContainer] =
    React.useState<HTMLElement | null>(null);

  React.useEffect(() => {
    if (Platform.OS === "web" && portalHost) {
      const element = document.querySelector<HTMLElement>(
        `[data-portal-host="${portalHost}"]`,
      );
      setPortalContainer(element);
    }
  }, [portalHost]);

  return (
    <DropdownMenuPrimitive.Portal
      hostName={portalHost}
      {...(Platform.OS === "web" && portalContainer
        ? { container: portalContainer }
        : {})}
    >
      <DropdownMenuPrimitive.Overlay
        style={[
          Platform.OS !== "web" ? StyleSheet.absoluteFill : undefined,
          overlayStyle,
        ]}
      >
        <DropdownMenuPrimitive.Content
          ref={ref}
          style={
            [
              a.zIndex[50],
              a.sizes.minWidth[64],
              { backgroundColor: "transparent" },
            ] as any
          }
          {...props}
        >
          <AnimatedDropdownPanel maxHeight={maxHeight} style={style}>
            <ScrollView
              showsVerticalScrollIndicator={false}
              contentContainerStyle={p[2]}
            >
              {typeof children === "function"
                ? children({ pressed: false })
                : children}
            </ScrollView>
          </AnimatedDropdownPanel>
        </DropdownMenuPrimitive.Content>
      </DropdownMenuPrimitive.Overlay>
    </DropdownMenuPrimitive.Portal>
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
    { overlayStyle, maxHeightPercentage = 0.9, children, style, ...props },
    ref,
  ) => {
    const { theme } = useTheme();
    const { height } = useWindowDimensions();
    const maxHeight = height * maxHeightPercentage;

    return (
      <DropdownMenuPrimitive.Overlay
        style={[
          Platform.OS !== "web" ? StyleSheet.absoluteFill : undefined,
          overlayStyle,
        ]}
      >
        <DropdownMenuPrimitive.Content
          ref={ref}
          style={
            [
              { zIndex: 999999 },
              a.sizes.minWidth[64],
              a.sizes.maxWidth[64],
              { maxHeight: maxHeight },
              a.radius.all.md,
              a.borders.width.thin,
              { borderColor: theme.colors.border },
              { backgroundColor: theme.colors.popover },
              p[2],
              a.shadows.md,
              style,
            ] as any
          }
          {...props}
        >
          <ScrollView showsVerticalScrollIndicator={false}>
            {typeof children === "function"
              ? children({ pressed: false })
              : children}
          </ScrollView>
        </DropdownMenuPrimitive.Content>
      </DropdownMenuPrimitive.Overlay>
    );
  },
);

export const ResponsiveDropdownMenuContent = forwardRef<
  any,
  any & { onModeChange?: (isSheet: boolean) => void }
>(({ children, onModeChange, ...props }, ref) => {
  const { width } = useWindowDimensions();

  const isBottomSheet =
    Platform.OS !== "web" || (Platform.OS === "web" && width <= 980);

  React.useEffect(() => {
    onModeChange?.(isBottomSheet);
  }, [isBottomSheet, onModeChange]);

  if (isBottomSheet) {
    return (
      <DropdownMenuContent align="start" ref={ref} {...props}>
        {children}
      </DropdownMenuContent>
    );
  }
  return (
    <DropdownMenuContent ref={ref} {...props}>
      {children}
    </DropdownMenuContent>
  );
});

export const DropdownMenuItem = forwardRef<
  any,
  DropdownMenuPrimitive.ItemProps & {
    inset?: boolean;
    disabled?: boolean;
    onFocus?: (e: any) => void;
    onBlur?: (e: any) => void;
    // Opt out of the built-in row fill for items that paint their own hover
    // background (e.g. the Create / account menus with icon-recoloring rows).
    noHighlight?: boolean;
  }
>(
  (
    { inset, disabled, style, children, onFocus, onBlur, noHighlight, ...props },
    ref,
  ) => {
  const { theme } = useTheme();
  const c = theme.colors;
  // Highlight the row on hover and on keyboard focus. Radix moves DOM focus to
  // whichever item the pointer is over (roving focus), so a single `active`
  // flag covers both mouse and keyboard, and the surface fill replaces the
  // global :focus-visible outline that would otherwise draw an indigo ring.
  const [active, setActive] = React.useState(false);
  const highlight = active && !disabled && !noHighlight;
  return (
    <DropdownMenuPrimitive.Item
      ref={ref}
      // Flatten to a single style object: on web this style is forwarded
      // straight to a DOM node (via Radix asChild/Slot), and React DOM throws
      // "Indexed property setter is not supported" if handed a style array.
      style={StyleSheet.flatten([{ outlineStyle: "none" }, style]) as any}
      onFocus={(e: any) => {
        setActive(true);
        onFocus?.(e);
      }}
      onBlur={(e: any) => {
        setActive(false);
        onBlur?.(e);
      }}
      {...props}
    >
      <TextClassContext.Provider
        value={objectFromObjects([
          { color: theme.colors.popoverForeground },
          a.fontSize.base,
        ])}
      >
        <View
          onPointerEnter={() => setActive(true)}
          onPointerLeave={() => setActive(false)}
          style={[
            a.layout.flex.row,
            a.layout.flex.alignCenter,
            a.radius.all.sm,
            py[1],
            pl[2],
            pr[2],
            { backgroundColor: highlight ? c.surface3 : "transparent" },
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
    </DropdownMenuPrimitive.Item>
  );
  },
);

export const DropdownMenuCheckboxItem = forwardRef<
  any,
  DropdownMenuPrimitive.CheckboxItemProps & {
    ref?: React.RefObject<DropdownMenuPrimitive.CheckboxItemRef>;
    children?: React.ReactNode;
  }
>(({ children, checked, ...props }, ref) => {
  const { theme } = useTheme();
  return (
    <DropdownMenuPrimitive.CheckboxItem
      ref={ref}
      checked={checked}
      closeOnPress={props.closeOnPress || false}
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
          {checked && (
            <Check size={14} strokeWidth={3} color={theme.colors.foreground} />
          )}
        </View>
      </View>
    </DropdownMenuPrimitive.CheckboxItem>
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

  return (
    <DropdownMenuPrimitive.RadioItem
      ref={ref}
      closeOnPress={props.closeOnPress || false}
      value={value}
      {...props}
    >
      <View
        style={[
          a.layout.flex.row,
          a.layout.flex.alignCenter,
          a.radius.all.sm,
          py[1],
          pl[2],
          pr[1],
        ]}
      >
        <View style={[pl[1], layout.position.absolute, right[1]]}>
          <DropdownMenuPrimitive.ItemIndicator>
            <Check size={14} strokeWidth={3} color={theme.colors.foreground} />
          </DropdownMenuPrimitive.ItemIndicator>
        </View>
        {children}
      </View>
    </DropdownMenuPrimitive.RadioItem>
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
  { inset?: boolean; title?: string; description?: string; children: ReactNode }
>((props, ref) => {
  const { theme } = useTheme();
  const { inset, title, children, description, ...rest } = props;
  return (
    <View style={[inset && gap[2]]} ref={ref} {...rest}>
      {title && (
        <Text
          style={[
            { color: theme.colors.text },
            description ? pt[1] : py[1],
            pl[2],
          ]}
        >
          {title}
        </Text>
      )}
      {description && (
        <Text
          style={[{ color: theme.colors.textMuted }, pb[2], pl[2], fontSize.sm]}
        >
          {description}
        </Text>
      )}
      <View
        style={[
          { backgroundColor: theme.colors.muted },
          Platform.OS === "web" ? [px[2], py[1]] : p[2],
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

// Re-export DropdownMenuBottomSheet for compatibility with native
export const DropdownMenuBottomSheet = DropdownMenuContent;
