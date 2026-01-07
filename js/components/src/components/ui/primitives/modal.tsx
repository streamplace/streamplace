import React, { forwardRef } from "react";
import {
  Dimensions,
  GestureResponderEvent,
  Modal,
  ModalProps,
  Platform,
  ScrollView,
  ScrollViewProps,
  StyleSheet,
  TouchableOpacity,
  TouchableOpacityProps,
  View,
  ViewProps,
} from "react-native";

const { width: screenWidth, height: screenHeight } = Dimensions.get("window");

// Base modal primitive interface
export interface ModalPrimitiveProps extends Omit<ModalProps, "children"> {
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  children?: React.ReactNode;
}

// Modal root primitive - handles the native Modal component
export const ModalRoot = forwardRef<View, ModalPrimitiveProps>(
  (
    {
      open = false,
      onOpenChange,
      children,
      onRequestClose,
      animationType = "fade",
      presentationStyle = Platform.OS === "ios" ? "pageSheet" : "formSheet",
      statusBarTranslucent = Platform.OS !== "ios",
      transparent = true,
      ...props
    },
    ref,
  ) => {
    const handleRequestClose = React.useCallback(
      (e: any) => {
        if (onOpenChange) {
          onOpenChange(false);
        }
        if (onRequestClose) {
          onRequestClose(e);
        }
      },
      [onOpenChange, onRequestClose],
    );

    return (
      <Modal
        visible={open}
        onRequestClose={handleRequestClose}
        animationType={animationType}
        presentationStyle={presentationStyle}
        statusBarTranslucent={statusBarTranslucent}
        transparent={transparent}
        {...props}
      >
        <View ref={ref} style={primitiveStyles.container}>
          {children}
        </View>
      </Modal>
    );
  },
);

ModalRoot.displayName = "ModalRoot";

// Modal overlay primitive - semi-transparent background
export interface ModalOverlayProps extends TouchableOpacityProps {
  dismissible?: boolean;
  onDismiss?: () => void;
}

export const ModalOverlay = forwardRef<
  React.ElementRef<typeof TouchableOpacity>,
  ModalOverlayProps
>(
  (
    {
      dismissible = true,
      onDismiss,
      onPress,
      style,
      children,
      activeOpacity = 1,
      ...props
    },
    ref,
  ) => {
    const handlePress = React.useCallback(
      (event: GestureResponderEvent) => {
        if (dismissible && onDismiss) {
          onDismiss();
        }
        if (onPress) {
          onPress(event);
        }
      },
      [dismissible, onDismiss, onPress],
    );

    return (
      <TouchableOpacity
        ref={ref}
        style={[primitiveStyles.overlay, style]}
        activeOpacity={activeOpacity}
        onPress={handlePress}
        {...props}
      >
        {children}
      </TouchableOpacity>
    );
  },
);

ModalOverlay.displayName = "ModalOverlay";

// Modal content primitive - the actual content container
export interface ModalContentProps extends ViewProps {
  position?: "center" | "top" | "bottom" | "left" | "right";
  size?: "sm" | "md" | "lg" | "xl" | "full";
}

export const ModalContent = forwardRef<View, ModalContentProps>(
  (
    {
      children,
      position = "center",
      size = "md",
      style,
      onStartShouldSetResponder,
      ...props
    },
    ref,
  ) => {
    // Prevent touches from propagating to overlay
    const handleStartShouldSetResponder = React.useCallback(() => {
      return true;
    }, []);

    const positionStyle = React.useMemo(() => {
      switch (position) {
        case "top":
          return primitiveStyles.contentTop;
        case "bottom":
          return primitiveStyles.contentBottom;
        case "left":
          return primitiveStyles.contentLeft;
        case "right":
          return primitiveStyles.contentRight;
        case "center":
        default:
          return primitiveStyles.contentCenter;
      }
    }, [position]);

    const sizeStyle = React.useMemo(() => {
      switch (size) {
        case "sm":
          return primitiveStyles.sizeSm;
        case "lg":
          return primitiveStyles.sizeLg;
        case "xl":
          return primitiveStyles.sizeXl;
        case "full":
          return primitiveStyles.sizeFull;
        case "md":
        default:
          return primitiveStyles.sizeMd;
      }
    }, [size]);

    return (
      <View
        ref={ref}
        style={[primitiveStyles.content, positionStyle, sizeStyle, style]}
        onStartShouldSetResponder={
          onStartShouldSetResponder || handleStartShouldSetResponder
        }
        {...props}
      >
        {children}
      </View>
    );
  },
);

ModalContent.displayName = "ModalContent";

// Modal header primitive
export interface ModalHeaderProps extends ViewProps {
  withBorder?: boolean;
}

export const ModalHeader = forwardRef<View, ModalHeaderProps>(
  ({ children, withBorder = false, style, ...props }, ref) => {
    return (
      <View
        ref={ref}
        style={[
          primitiveStyles.header,
          withBorder && primitiveStyles.headerBorder,
          style,
        ]}
        {...props}
      >
        {children}
      </View>
    );
  },
);

ModalHeader.displayName = "ModalHeader";

// Modal body primitive - scrollable content area
export interface ModalBodyProps extends ScrollViewProps {
  scrollable?: boolean;
}

export const ModalBody = forwardRef<ScrollView, ModalBodyProps>(
  ({ children, scrollable = true, style, ...props }, ref) => {
    if (!scrollable) {
      return <View style={[primitiveStyles.body, style]}>{children}</View>;
    }

    return (
      <ScrollView
        ref={ref}
        style={[primitiveStyles.body, style]}
        showsVerticalScrollIndicator={false}
        keyboardShouldPersistTaps="handled"
        {...props}
      >
        {children}
      </ScrollView>
    );
  },
);

ModalBody.displayName = "ModalBody";

// Modal footer primitive
export interface ModalFooterProps extends ViewProps {
  withBorder?: boolean;
  direction?: "row" | "column";
  justify?:
    | "flex-start"
    | "center"
    | "flex-end"
    | "space-between"
    | "space-around";
}

export const ModalFooter = forwardRef<View, ModalFooterProps>(
  (
    {
      children,
      withBorder = false,
      direction = "row",
      justify = "flex-end",
      style,
      ...props
    },
    ref,
  ) => {
    return (
      <View
        ref={ref}
        style={[
          primitiveStyles.footer,
          withBorder && primitiveStyles.footerBorder,
          {
            flexDirection: direction,
            justifyContent: justify,
          },
          style,
        ]}
        {...props}
      >
        {children}
      </View>
    );
  },
);

ModalFooter.displayName = "ModalFooter";

// Modal close trigger primitive
export interface ModalCloseProps extends TouchableOpacityProps {
  onClose?: () => void;
}

export const ModalClose = forwardRef<
  React.ElementRef<typeof TouchableOpacity>,
  ModalCloseProps
>(({ children, onClose, onPress, ...props }, ref) => {
  const handlePress = React.useCallback(
    (event: GestureResponderEvent) => {
      if (onClose) {
        onClose();
      }
      if (onPress) {
        onPress(event);
      }
    },
    [onClose, onPress],
  );

  return (
    <TouchableOpacity ref={ref} onPress={handlePress} {...props}>
      {children}
    </TouchableOpacity>
  );
});

ModalClose.displayName = "ModalClose";

// Primitive styles (minimal, unstyled)
const primitiveStyles = StyleSheet.create({
  container: {
    flex: 1,
  },
  overlay: {
    flex: 1,
    backgroundColor: "rgba(0, 0, 0, 0.5)",
    justifyContent: "center",
    alignItems: "center",
    padding: 16,
  },
  content: {
    borderRadius: 8,
    overflow: "hidden",
  },
  contentCenter: {
    alignSelf: "center",
  },
  contentTop: {
    alignSelf: "center",
    marginTop: 0,
  },
  contentBottom: {
    alignSelf: "center",
    marginTop: "auto",
  },
  contentLeft: {
    alignSelf: "flex-start",
    marginRight: "auto",
  },
  contentRight: {
    alignSelf: "flex-end",
    marginLeft: "auto",
  },
  sizeSm: {
    maxWidth: screenWidth * 0.4,
    maxHeight: screenHeight * 0.6,
  },
  sizeMd: {
    maxWidth: screenWidth * 0.6,
    maxHeight: screenHeight * 0.8,
  },
  sizeLg: {
    maxWidth: screenWidth * 0.8,
    maxHeight: screenHeight * 0.9,
  },
  sizeXl: {
    maxWidth: screenWidth * 0.95,
    maxHeight: screenHeight * 0.95,
  },
  sizeFull: {
    width: screenWidth,
    height: screenHeight,
    maxWidth: screenWidth,
    maxHeight: screenHeight,
    borderRadius: 0,
  },
  header: {
    paddingHorizontal: 16,
    paddingVertical: 12,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  headerBorder: {
    borderBottomWidth: 1,
    borderBottomColor: "#e5e7eb",
  },
  body: {
    flex: 1,
    paddingHorizontal: 16,
    paddingVertical: 12,
  },
  footer: {
    paddingHorizontal: 16,
    paddingVertical: 12,
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  footerBorder: {
    borderTopWidth: 1,
    borderTopColor: "#e5e7eb",
  },
});

// Export primitive collection
export const ModalPrimitive = {
  Root: ModalRoot,
  Overlay: ModalOverlay,
  Content: ModalContent,
  Header: ModalHeader,
  Body: ModalBody,
  Footer: ModalFooter,
  Close: ModalClose,
};
