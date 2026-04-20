import React from "react";
import { PinnedRecordViewHydrated } from "streamplace";
import { streamNotification } from "../components/stream-notification";
import { PinnedCommentNotification } from "../components/stream-notification/pin-notification";
import { TeleportNotification } from "../components/stream-notification/teleport-notification";

export const StreamNotifications = {
  pinnedComment: (params: {
    pinnedComment: PinnedRecordViewHydrated;
    onDismiss?: (reason?: "user" | "auto") => void;
    onUnpin?: () => void;
  }) => {
    streamNotification.show({
      id: "pinned-comment",
      render: (isExiting, onDismiss) => {
        return React.createElement(PinnedCommentNotification, {
          pinnedComment: params.pinnedComment,
          onDismiss: () => onDismiss("user"),
          onUnpin: () => {
            params.onUnpin?.();
            onDismiss("user");
          },
        });
      },
      duration: 0, // manually dismissed or auto-dismissed by TTL
      onDismiss: params.onDismiss,
    });
  },

  pinnedCommentDismiss: () => {
    streamNotification.hide("pinned-comment");
  },

  teleport: (params: {
    targetHandle: string;
    targetDID: string;
    countdown: number;
    canCancel: boolean;
    onDismiss?: (reason?: "user" | "auto") => void;
  }) => {
    streamNotification.show({
      id: "teleport",
      render: (isExiting, onDismiss, startTime) => {
        return React.createElement(TeleportNotification, {
          targetHandle: params.targetHandle,
          countdown: params.countdown,
          canCancel: params.canCancel,
          startTime: startTime,
          onDismiss: onDismiss,
        });
      },
      duration: 0, // manually dismissed by countdown or user cancel
      variant: "warning",
      onDismiss: params.onDismiss,
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

  activate: (message: string) => {
    streamNotification.show({
      id: "stream-activate",
      message: message,
      duration: 3,
      variant: "info",
    });
  },
};
