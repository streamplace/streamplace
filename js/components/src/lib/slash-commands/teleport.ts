import { PlaceStreamLiveTeleport, StreamplaceAgent } from "streamplace";
import { registerSlashCommand, SlashCommandResult } from "../slash-commands";

export function registerTeleportCommand(
  pdsAgent: StreamplaceAgent,
  userDID: string,
) {
  registerSlashCommand({
    name: "teleport",
    description: "Start a teleport to another streamer",
    usage: "/teleport @handle.bsky.social [duration_seconds]",
    handler: async (args, rawInput): Promise<SlashCommandResult> => {
      if (args.length === 0) {
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

      let durationSeconds: number | undefined;
      if (args.length > 1) {
        const parsedDuration = parseInt(args[1], 10);
        if (isNaN(parsedDuration)) {
          return {
            handled: true,
            error: "Duration must be a number (seconds)",
          };
        }
        if (parsedDuration < 60 || parsedDuration > 32400) {
          return {
            handled: true,
            error:
              "Duration must be between 60 seconds and 32400 seconds (9 hours)",
          };
        }
        durationSeconds = parsedDuration;
      }

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

      const startsAt = new Date(Date.now() + 30000).toISOString();

      const record: PlaceStreamLiveTeleport.Record = {
        $type: "place.stream.live.teleport",
        streamer: targetDID,
        startsAt,
        ...(durationSeconds ? { durationSeconds } : {}),
      };

      try {
        await pdsAgent.com.atproto.repo.createRecord({
          repo: userDID,
          collection: "place.stream.live.teleport",
          record,
        });

        return { handled: true };
      } catch (err) {
        return {
          handled: true,
          error:
            err instanceof Error ? err.message : "Failed to create teleport",
        };
      }
    },
  });
}
