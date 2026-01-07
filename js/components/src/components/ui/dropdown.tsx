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
>(({ inset, children, subMenuTitle, ...props }, ref) => {
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
  DropdownMenuPrimitive.SubContentProps & { children?: ReactNode }
>(({ children, ...props }, ref) => {
  const { zero: zt } = useTheme();

  return (
    <DropdownMenuPrimitive.SubContent
      ref={ref}
      style={[
        a.zIndex[50],
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
      ]}
      {...props}
    >
      {children}
    </DropdownMenuPrimitive.SubContent>
  );
});

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

  return (
    <DropdownMenuPrimitive.Portal hostName={portalHost}>
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
              a.sizes.maxWidth[64],
              { maxHeight: maxHeight },
              a.overflow.hidden,
              a.radius.all.md,
              a.borders.width.thin,
              zt.border.default,
              zt.bg.popover,
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
  DropdownMenuPrimitive.ItemProps & { inset?: boolean; disabled?: boolean }
>(({ inset, disabled, style, children, ...props }, ref) => {
  const { theme } = useTheme();
  return (
    <DropdownMenuPrimitive.Item ref={ref} {...props}>
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
    </DropdownMenuPrimitive.Item>
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
          <DropdownMenuPrimitive.ItemIndicator>
            <Check size={14} strokeWidth={3} color={theme.colors.foreground} />
          </DropdownMenuPrimitive.ItemIndicator>
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
