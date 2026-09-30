import { place, StreamplaceAgent } from "streamplace";
import {
  registerSlashCommand,
  SlashCommandHandler,
  SlashCommandResult,
} from "../slash-commands";

const TELEPORT_ERROR_CODES = [
  "streamer-only",
  "handle-format",
  "countdown-number",
  "countdown-range",
  "self",
  "resolve-handle",
  "create",
] as const;

export type TeleportErrorCode = (typeof TELEPORT_ERROR_CODES)[number];

export type TeleportErrorData = {
  code: TeleportErrorCode;
  params?: { handle?: string };
};

export function isTeleportErrorCode(code: string): code is TeleportErrorCode {
  return TELEPORT_ERROR_CODES.some((knownCode) => knownCode === code);
}

function teleportError(
  message: string,
  code: TeleportErrorCode,
  params?: TeleportErrorData["params"],
): SlashCommandResult {
  return {
    handled: true,
    error: message,
    errorData: {
      type: "teleport",
      code,
      ...(params ? { params } : {}),
    },
  };
}

export async function deleteTeleport(
  pdsAgent: StreamplaceAgent,
  userDID: string,
  uri: string,
) {
  const rkey = uri.split("/").pop();
  if (!rkey) {
    throw new Error("No rkey found in teleport URI");
  }
  return await pdsAgent.client.delete(place.stream.live.teleport, {
    repo: userDID as any,
    rkey: rkey,
  });
}

export async function createTeleport(
  pdsAgent: StreamplaceAgent,
  userDID: string,
  targetHandle: string,
  countdownSeconds: number,
  livestream?: { uri: string; cid: string } | null,
  onTeleportCreated?: (uri: string, targetDID: string) => void,
): Promise<{ success: boolean; error?: string }> {
  if (countdownSeconds < 5 || countdownSeconds > 300) {
    return {
      success: false,
      error: "Countdown must be between 5 seconds and 5 minutes",
    };
  }

  let targetDID: string;
  try {
    const resolution = await pdsAgent.resolveHandle({
      handle: targetHandle,
    });
    targetDID = resolution.data.did;
  } catch (err) {
    return {
      success: false,
      error: `Could not resolve handle: ${targetHandle}`,
    };
  }

  if (targetDID === userDID) {
    return {
      success: false,
      error: "You cannot teleport to yourself",
    };
  }

  const startsAt = new Date(Date.now() + countdownSeconds * 1000).toISOString();

  // The `livestream` strongRef is optional: when present it pins the source
  // stream so the server can end exactly that record on arrival. When absent
  // (e.g. no active livestream) the teleport still sends viewers over; the
  // server just won't end a source stream.
  const record: Record<string, unknown> = {
    streamer: targetDID,
    startsAt: startsAt,
  };
  if (livestream?.uri && livestream?.cid) {
    record.livestream = { uri: livestream.uri, cid: livestream.cid };
  }

  try {
    const result = await pdsAgent.client.create(
      place.stream.live.teleport,
      record as any,
      { repo: userDID as any },
    );

    onTeleportCreated?.(result.uri, targetDID);

    return { success: true };
  } catch (err) {
    return {
      success: false,
      error: err instanceof Error ? err.message : "Failed to create teleport",
    };
  }
}

export function registerTeleportCommand(
  pdsAgent: StreamplaceAgent,
  userDID: string,
  getLivestream?: () => {
    uri: string;
    cid: string;
    streamerDid: string;
  } | null,
  onTeleportCreated?: (uri: string, targetDID: string) => void,
  onOpenModal?: () => void,
): () => void {
  const teleportHandler: SlashCommandHandler = async (
    args,
    rawInput,
  ): Promise<SlashCommandResult> => {
    const livestream = getLivestream?.() ?? null;
    if (!livestream || livestream.streamerDid !== userDID) {
      return teleportError(
        "Only the streamer of the current livestream can start a teleport",
        "streamer-only",
      );
    }

    if (args.length === 0) {
      if (onOpenModal) {
        onOpenModal();
        return { handled: true };
      }
      return {
        handled: true,
        error: "Usage: /teleport @handle.bsky.social [duration_seconds]",
      };
    }

    let targetHandle = args[0];

    if (targetHandle.startsWith("@")) {
      targetHandle = targetHandle.slice(1);
    }

    if (!targetHandle.includes(".")) {
      return teleportError(
        "Invalid handle format. Expected: handle.bsky.social",
        "handle-format",
      );
    }

    let countdownSeconds = 10;
    if (args.length > 1) {
      const parsedDuration = parseInt(args[1], 10);
      if (isNaN(parsedDuration)) {
        return teleportError(
          "Countdown must be a number (seconds)",
          "countdown-number",
        );
      }
      if (parsedDuration < 5 || parsedDuration > 300) {
        return teleportError(
          "Countdown must be between 5 seconds and 5 minutes",
          "countdown-range",
        );
      }
      countdownSeconds = parsedDuration;
    }

    // The `livestream` strongRef is optional: when present it pins the source
    // stream so the server can end exactly that record on arrival. When absent
    // the teleport still sends viewers over; the server just won't end a
    // source stream.
    let targetDID: string;
    try {
      const resolution = await pdsAgent.resolveHandle({
        handle: targetHandle,
      });
      targetDID = resolution.data.did;
    } catch (err) {
      return teleportError(
        `Could not resolve handle: ${targetHandle}`,
        "resolve-handle",
        { handle: targetHandle },
      );
    }

    if (targetDID === userDID) {
      return teleportError("You cannot teleport to yourself", "self");
    }

    const startsAt = new Date(
      Date.now() + countdownSeconds * 1000,
    ).toISOString();

    const record: Record<string, unknown> = {
      streamer: targetDID,
      startsAt: startsAt,
    };
    if (livestream?.uri && livestream?.cid) {
      record.livestream = { uri: livestream.uri, cid: livestream.cid };
    }

    try {
      const result = await pdsAgent.client.create(
        place.stream.live.teleport,
        record as any,
        { repo: userDID as any },
      );

      onTeleportCreated?.(result.uri, targetDID);

      return { handled: true };
    } catch (err) {
      return teleportError(
        err instanceof Error ? err.message : "Failed to create teleport",
        "create",
      );
    }
  };

  const unregisterTeleport = registerSlashCommand({
    name: "teleport",
    description: "Start a teleport to another streamer",
    usage: "/teleport @handle.bsky.social [duration_seconds]",
    handler: teleportHandler,
  });

  const unregisterTp = registerSlashCommand({
    name: "tp",
    description: "Start a teleport to another streamer (alias for /teleport)",
    usage: "/tp @handle.bsky.social [duration_seconds]",
    handler: teleportHandler,
  });

  return () => {
    unregisterTeleport();
    unregisterTp();
  };
}
