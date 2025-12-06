import React from "react";
import { streamNotification } from "../components/stream-notification";
import { TeleportNotification } from "../components/stream-notification/teleport-notification";

export const StreamNotifications = {
  teleport: (params: {
    targetHandle: string;
    targetDID: string;
    countdown: number;
    onCancel?: () => void;
    onAutoDismiss?: () => void;
  }) => {
    streamNotification.show({
      id: "teleport",
      render: (isExiting, onDismiss, startTime) => {
        return React.createElement(TeleportNotification, {
          targetHandle: params.targetHandle,
          countdown: params.countdown,
          startTime: startTime,
          onDismiss: onDismiss,
        });
      },
      duration: 0, // manually dismissed by countdown or user cancel
      variant: "warning",
      onUserDismiss: params.onCancel,
      onAutoDismiss: params.onAutoDismiss,
    });
  },

  teleportCancelled: () => {
    streamNotification.hide("teleport");
  },

  teleportNow: (targetHandle: string) => {
    streamNotification.show({
      id: "teleport-now",
      message: `Teleporting to @${targetHandle}...`,
      duration: 2,
      variant: "info",
    });
  },
};
