import React from "react";
import { streamNotification } from "../components/stream-notification";
import { TeleportNotification } from "../components/stream-notification/teleport-notification";

export const StreamNotifications = {
  teleport: (params: {
    targetHandle: string;
    targetDID: string;
    countdown: number;
    onCancel?: () => void;
  }) => {
    streamNotification.show({
      id: "teleport",
      render: (isExiting, onDismiss) => {
        return React.createElement(TeleportNotification, {
          targetHandle: params.targetHandle,
          countdown: params.countdown,
          onDismiss: () => {
            params.onCancel?.();
            onDismiss();
          },
        });
      },
      // allow some extra time for the countdown animation to finish
      duration: 30 + 7,
      variant: "warning",
      onUserDismiss: params.onCancel,
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
