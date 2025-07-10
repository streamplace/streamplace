import { cva, type VariantProps } from "class-variance-authority";
import { X } from "lucide-react-native";
import React, { forwardRef } from "react";
import { Platform, StyleSheet, Text } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import { createThemedIcon } from "./icons";
import { ModalPrimitive, ModalPrimitiveProps } from "./primitives/modal";

const ThemedX = createThemedIcon(X);

// Dialog variants using class-variance-authority pattern
const dialogVariants = cva("", {
  variants: {
    variant: {
      default: "default",
      sheet: "sheet",
      fullscreen: "fullscreen",
    },
    size: {
      sm: "sm",
      md: "md",
      lg: "lg",
      xl: "xl",
      full: "full",
    },
    position: {
      center: "center",
      top: "top",
      bottom: "bottom",
      left: "left",
      right: "right",
    },
  },
  defaultVariants: {
    variant: "default",
    size: "md",
    position: "center",
  },
});

export interface DialogProps
  extends Omit<ModalPrimitiveProps, "children">,
    VariantProps<typeof dialogVariants> {
  children?: React.ReactNode;
  title?: string;
  description?: string;
  dismissible?: boolean;
  showCloseButton?: boolean;
  onClose?: () => void;
}

export const Dialog = forwardRef<any, DialogProps>(
  (
    {
      variant = "left",
      size = "md",
      position = "center",
      children,
      title,
      description,
      dismissible = true,
      showCloseButton = true,
      onClose,
      open = false,
      onOpenChange,
      ...props
    },
    ref,
  ) => {
    const { theme } = useTheme();

    // Create dynamic styles based on theme
    const styles = React.useMemo(() => createStyles(theme), [theme]);

    const handleClose = React.useCallback(() => {
      if (onClose) {
        onClose();
      }
      if (onOpenChange) {
        onOpenChange(false);
      }
    }, [onClose, onOpenChange]);

    const presentationStyle = React.useMemo(() => {
      if (variant === "sheet" && Platform.OS === "ios") {
        return "pageSheet" as const;
      }
      if (variant === "fullscreen") {
        return "fullScreen" as const;
      }
      return Platform.OS === "ios"
        ? ("pageSheet" as const)
        : ("fullScreen" as const);
    }, [variant]);

    const animationType = React.useMemo(() => {
      if (variant === "sheet") {
        return "slide" as const;
      }
      return "fade" as const;
    }, [variant]);

    return (
      <ModalPrimitive.Root
        ref={ref}
        open={open}
        onOpenChange={onOpenChange}
        presentationStyle={presentationStyle}
        animationType={animationType}
        {...props}
      >
        <ModalPrimitive.Overlay
          dismissible={dismissible}
          onDismiss={handleClose}
          style={styles.overlay}
        >
          <ModalPrimitive.Content
            position={position || "left"}
            size={size || "md"}
            style={[
              styles.content,
              styles[`${variant}Content` as keyof typeof styles],
              styles[`${size}Content` as keyof typeof styles],
            ]}
          >
            {(title || showCloseButton) && (
              <ModalPrimitive.Header
                withBorder={variant !== "sheet"}
                style={styles.header}
              >
                <DialogTitle>{title}</DialogTitle>
                {showCloseButton && (
                  <ModalPrimitive.Close
                    onClose={handleClose}
                    style={styles.closeButton}
                  >
                    <DialogCloseIcon />
                  </ModalPrimitive.Close>
                )}
              </ModalPrimitive.Header>
            )}

            <ModalPrimitive.Body
              scrollable={variant !== "fullscreen"}
              style={styles.body}
            >
              {description && (
                <DialogDescription>{description}</DialogDescription>
              )}
              {children}
            </ModalPrimitive.Body>
          </ModalPrimitive.Content>
        </ModalPrimitive.Overlay>
      </ModalPrimitive.Root>
    );
  },
);

Dialog.displayName = "Dialog";

// Dialog Title component
export interface DialogTitleProps {
  children?: React.ReactNode;
  style?: any;
}

export const DialogTitle = forwardRef<any, DialogTitleProps>(
  ({ children, style, ...props }, ref) => {
    const { theme } = useTheme();
    const styles = React.useMemo(() => createStyles(theme), [theme]);

    if (!children) return null;

    return (
      <Text ref={ref} style={[styles.title, style]} {...props}>
        {children}
      </Text>
    );
  },
);

DialogTitle.displayName = "DialogTitle";

// Dialog Description component
export interface DialogDescriptionProps {
  children?: React.ReactNode;
  style?: any;
}

export const DialogDescription = forwardRef<any, DialogDescriptionProps>(
  ({ children, style, ...props }, ref) => {
    const { theme } = useTheme();
    const styles = React.useMemo(() => createStyles(theme), [theme]);

    if (!children) return null;

    return (
      <Text ref={ref} style={[styles.description, style]} {...props}>
        {children}
      </Text>
    );
  },
);

DialogDescription.displayName = "DialogDescription";

// Dialog Footer component
export interface DialogFooterProps {
  children?: React.ReactNode;
  direction?: "row" | "column";
  justify?:
    | "flex-start"
    | "center"
    | "flex-end"
    | "space-between"
    | "space-around";
  withBorder?: boolean;
  style?: any;
}

export const DialogFooter = forwardRef<any, DialogFooterProps>(
  (
    {
      children,
      direction = "row",
      justify = "flex-end",
      withBorder = true,
      style,
      ...props
    },
    ref,
  ) => {
    const { theme } = useTheme();
    const styles = React.useMemo(() => createStyles(theme), [theme]);

    if (!children) return null;

    return (
      <ModalPrimitive.Footer
        ref={ref}
        withBorder={withBorder}
        direction={direction}
        justify={justify}
        style={[styles.footer, style]}
        {...props}
      >
        {children}
      </ModalPrimitive.Footer>
    );
  },
);

DialogFooter.displayName = "DialogFooter";

// Dialog Close Icon component (Lucide X)
const DialogCloseIcon = () => {
  return <ThemedX size="md" variant="default" />;
};

// Create theme-aware styles
function createStyles(theme: any) {
  return StyleSheet.create({
    overlay: {
      backgroundColor: "rgba(0, 0, 0, 0.5)",
    },

    content: {
      backgroundColor: theme.colors.card,
      borderRadius: theme.borderRadius.lg,
      ...theme.shadows.lg,
      maxHeight: "90%",
      maxWidth: "90%",
    },

    // Variant styles
    defaultContent: {
      // Default styles already applied in content
    },

    sheetContent: {
      borderTopLeftRadius: theme.borderRadius.xl,
      borderTopRightRadius: theme.borderRadius.xl,
      borderBottomLeftRadius: 0,
      borderBottomRightRadius: 0,
      marginTop: "auto",
      marginBottom: 0,
      maxHeight: "80%",
      width: "100%",
      maxWidth: "100%",
    },

    fullscreenContent: {
      width: "100%",
      height: "100%",
      maxWidth: "100%",
      maxHeight: "100%",
      borderRadius: 0,
      margin: 0,
    },

    // Size styles
    smContent: {
      minWidth: 300,
      minHeight: 200,
    },

    mdContent: {
      minWidth: 400,
      minHeight: 300,
    },

    lgContent: {
      minWidth: 500,
      minHeight: 400,
    },

    xlContent: {
      minWidth: 600,
      minHeight: 500,
    },

    fullContent: {
      width: "95%",
      height: "95%",
      maxWidth: "95%",
      maxHeight: "95%",
    },

    header: {
      paddingHorizontal: theme.spacing[6],
      paddingVertical: theme.spacing[4],
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
    },

    body: {
      paddingHorizontal: theme.spacing[6],
      paddingBottom: theme.spacing[6],
      flex: 1,
    },

    footer: {
      paddingHorizontal: theme.spacing[6],
      paddingVertical: theme.spacing[4],
      gap: theme.spacing[2],
    },

    title: {
      fontSize: 20,
      fontWeight: "600",
      color: theme.colors.text,
      flex: 1,
      lineHeight: 24,
    },

    description: {
      fontSize: 16,
      color: theme.colors.textMuted,
      lineHeight: 22,
      marginBottom: theme.spacing[4],
    },

    closeButton: {
      width: theme.touchTargets.minimum,
      height: theme.touchTargets.minimum,
      alignItems: "center",
      justifyContent: "center",
      borderRadius: theme.borderRadius.sm,
      marginLeft: theme.spacing[2],
    },
  });
}

// Export dialog variants for external use
export { dialogVariants };
