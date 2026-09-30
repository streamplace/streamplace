import type { StreamplaceAgent } from "streamplace";
import { afterEach, describe, expect, it, vi } from "vitest";
import { handleSlashCommand } from "../slash-commands";
import { registerTeleportCommand } from "./teleport";

describe("teleport slash command errors", () => {
  let unregister: (() => void) | undefined;

  afterEach(() => {
    unregister?.();
    unregister = undefined;
  });

  function register(agent: Partial<StreamplaceAgent>) {
    unregister = registerTeleportCommand(
      agent as StreamplaceAgent,
      "did:plc:streamer",
      () => ({
        uri: "at://did:plc:streamer/place.stream.livestream/stream",
        cid: "stream-cid",
        streamerDid: "did:plc:streamer",
      }),
    );
  }

  it("returns a stable code for invalid handles", async () => {
    register({});

    const result = await handleSlashCommand("/teleport invalid");

    expect(result.errorData).toEqual({
      type: "teleport",
      code: "handle-format",
    });
  });

  it("returns a stable code and handle when resolution fails", async () => {
    register({
      resolveHandle: vi.fn().mockRejectedValue(new Error("network error")),
    });

    const result = await handleSlashCommand("/teleport @missing.example");

    expect(result.errorData).toEqual({
      type: "teleport",
      code: "resolve-handle",
      params: { handle: "missing.example" },
    });
  });
});
