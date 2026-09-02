import { X } from "lucide-react-native";
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, View } from "react-native";
import Animated, {
  Easing,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { Text, useTheme } from "../../";
import { colors as tokensColors } from "../../lib/theme/tokens";
import {
  StreamNotification,
  streamNotificationManager,
} from "./stream-notification-manager";

export function StreamNotificationProvider({
  children = <></>,
  position = "top",
}: {
  children?: React.ReactNode;
  position?: "top" | "bottom";
}) {
  const [notifications, setNotifications] = useState(
    streamNotificationManager.getAll(),
  );

  useEffect(() => {
    return streamNotificationManager.subscribe(setNotifications);
  }, []);

  return (
    <View style={styles.container}>
      {children}
      {notifications.map((notification, index) => (
        <NotificationItem
          key={notification.id}
          notification={notification}
          index={index}
          position={position}
        />
      ))}
    </View>
  );
}

function NotificationItem({
  notification,
  index,
  position,
}: {
  notification: StreamNotification;
  index: number;
  position: "top" | "bottom";
}) {
  const { theme } = useTheme();
  const translateY = useSharedValue(position === "top" ? -100 : 100);
  const opacity = useSharedValue(0);
  const [isExiting, setIsExiting] = useState(false);

  const NOTIFICATION_HEIGHT = 60;
  const NOTIFICATION_GAP = 8;
  const offset = 16 + index * (NOTIFICATION_HEIGHT + NOTIFICATION_GAP);

  useEffect(() => {
    translateY.value = withTiming(position === "top" ? offset : -offset, {
      duration: 300,
      easing: Easing.out(Easing.cubic),
    });
    opacity.value = withTiming(1, {
      duration: 200,
    });
  }, [offset, position]);

  useEffect(() => {
    if (notification.shouldDismiss && !isExiting) {
      setIsExiting(true);
      setTimeout(() => {
        streamNotificationManager.hide(
          notification.id,
          notification.dismissReason || "auto",
        );
      }, 200);
    }
  }, [
    notification.shouldDismiss,
    isExiting,
    notification.id,
    notification.dismissReason,
  ]);

  useEffect(() => {
    if (isExiting) {
      translateY.value = withTiming(position === "top" ? -100 : 100, {
        duration: 200,
        easing: Easing.in(Easing.cubic),
      });
      opacity.value = withTiming(0, {
        duration: 200,
      });
    }
  }, [isExiting, position]);

  const animatedStyle = useAnimatedStyle(() => ({
    transform: [{ translateY: translateY.value }],
    opacity: opacity.value,
  }));

  const variantStyles = {
    default: {
      backgroundColor: theme.colors.card,
      borderColor: theme.colors.border,
    },
    info: {
      backgroundColor: theme.colors.info,
      borderColor: theme.colors.info,
    },
    warning: {
      backgroundColor: theme.colors.warning,
      borderColor: theme.colors.warning,
    },
  };

  const handleDismiss = (reason: "user" | "auto" = "user") => {
    console.log("Dismissing notification:", notification.id);
    setIsExiting(true);
    setTimeout(() => {
      console.log("Requesting dismiss for notification:", notification.id);
      streamNotificationManager.hide(notification.id, reason);
    }, 200);
    console.log(streamNotificationManager.getAll());
  };

  const handleAction = () => {
    notification.onAction?.();
    streamNotificationManager.hide(notification.id, "user");
  };

  const positionStyle = position === "top" ? { top: 0 } : { bottom: 0 };

  return (
    <Animated.View
      style={[
        styles.notification,
        positionStyle,
        notification.render
          ? {}
          : variantStyles[notification.variant || "default"],
        { margin: 0, padding: 0 },
        animatedStyle,
      ]}
    >
      {notification.render ? (
        notification.render(isExiting, handleDismiss, notification.startTime)
      ) : (
        <View style={styles.content}>
          <Text style={[styles.message, { color: theme.colors.foreground }]}>
            {notification.message}
          </Text>

          <View style={styles.actions}>
            {notification.actionLabel && (
              <Pressable onPress={handleAction}>
                <Text
                  style={[styles.actionButton, { color: theme.colors.primary }]}
                >
                  {notification.actionLabel}
                </Text>
              </Pressable>
            )}

            <Pressable
              onPress={() => handleDismiss("user")}
              style={styles.closeButton}
            >
              <X size={16} color={theme.colors.mutedForeground} />
            </Pressable>
          </View>
        </View>
      )}
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    pointerEvents: "box-none",
  },
  notification: {
    position: "absolute",
    top: 0,
    left: 16,
    right: 16,
    zIndex: 9999,
    borderRadius: 8,
    borderWidth: 1,
    padding: 12,
    shadowColor: tokensColors.black,
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.25,
    shadowRadius: 8,
    elevation: 5,
  },
  content: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 12,
  },
  message: {
    flex: 1,
    fontSize: 14,
    fontWeight: "500",
  },
  actions: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
  },
  actionButton: {
    fontSize: 14,
    fontWeight: "600",
  },
  closeButton: {
    padding: 4,
  },
});
