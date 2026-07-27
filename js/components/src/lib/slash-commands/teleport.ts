import { place, StreamplaceAgent } from "streamplace";
import {
  registerSlashCommand,
  SlashCommandHandler,
  SlashCommandResult,
} from "../slash-commands";

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
  setActiveTeleportUri?: (uri: string | null) => void,
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

    if (setActiveTeleportUri) {
      setActiveTeleportUri(result.uri);
    }

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
  getLivestream?: () => { uri: string; cid: string } | null,
  setActiveTeleportUri?: (uri: string | null) => void,
  onOpenModal?: () => void,
) {
  const teleportHandler: SlashCommandHandler = async (
    args,
    rawInput,
  ): Promise<SlashCommandResult> => {
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
      return {
        handled: true,
        error: "Invalid handle format. Expected: handle.bsky.social",
      };
    }

    let countdownSeconds = 10;
    if (args.length > 1) {
      const parsedDuration = parseInt(args[1], 10);
      if (isNaN(parsedDuration)) {
        return {
          handled: true,
          error: "Countdown must be a number (seconds)",
        };
      }
      if (parsedDuration < 5 || parsedDuration > 300) {
        return {
          handled: true,
          error: "Countdown must be between 5 seconds and 5 minutes",
        };
      }
      countdownSeconds = parsedDuration;
    }

    // The `livestream` strongRef is optional: when present it pins the source
    // stream so the server can end exactly that record on arrival. When absent
    // the teleport still sends viewers over; the server just won't end a
    // source stream.
    const livestream = getLivestream?.() ?? null;

    let targetDID: string;
    try {
      const resolution = await pdsAgent.resolveHandle({
        handle: targetHandle,
      });
      targetDID = resolution.data.did;
    } catch (err) {
      return {
        handled: true,
        error: `Could not resolve handle: ${targetHandle}`,
      };
    }

    if (targetDID === userDID) {
      return {
        handled: true,
        error: "You cannot teleport to yourself",
      };
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

      // store the URI in the livestream store
      if (setActiveTeleportUri) {
        setActiveTeleportUri(result.uri);
      }

      return { handled: true };
    } catch (err) {
      return {
        handled: true,
        error: err instanceof Error ? err.message : "Failed to create teleport",
      };
    }
  };

  registerSlashCommand({
    name: "teleport",
    description: "Start a teleport to another streamer",
    usage: "/teleport @handle.bsky.social [duration_seconds]",
    handler: teleportHandler,
  });

  registerSlashCommand({
    name: "tp",
    description: "Start a teleport to another streamer (alias for /teleport)",
    usage: "/tp @handle.bsky.social [duration_seconds]",
    handler: teleportHandler,
  });
}
