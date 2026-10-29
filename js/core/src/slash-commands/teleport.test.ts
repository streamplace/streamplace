import type { StreamplaceAgent } from "streamplace";
import { describe, expect, it, vi } from "vitest";
import { handleSlashCommand, type SlashCommand } from "../slash-commands";
import { createTeleport, createTeleportCommands } from "./teleport";

describe("teleport slash command errors", () => {
  let commands: SlashCommand[];

  function register(
    agent: Partial<StreamplaceAgent>,
    getLivestream: Parameters<typeof createTeleportCommands>[2] = () => ({
      uri: "at://did:plc:streamer/place.stream.livestream/stream",
      cid: "stream-cid",
      streamerDid: "did:plc:streamer",
    }),
  ) {
    commands = createTeleportCommands(
      agent as StreamplaceAgent,
      "did:plc:streamer",
      getLivestream,
    );
  }

  it("tells an offline streamer to start a livestream", async () => {
    register({}, () => null);

    const result = await handleSlashCommand(
      "/teleport @target.example",
      commands,
    );

    expect(result.errorData).toEqual({ type: "teleport", code: "not-live" });
  });

  it("distinguishes a viewer from an offline streamer", async () => {
    register({}, () => ({
      uri: "at://did:plc:other/place.stream.livestream/stream",
      cid: "other-stream-cid",
      streamerDid: "did:plc:other",
    }));

    const result = await handleSlashCommand(
      "/teleport @target.example",
      commands,
    );

    expect(result.errorData).toEqual({
      type: "teleport",
      code: "streamer-only",
    });
  });

  it("keeps commands scoped to the chat that invokes them", async () => {
    register({});
    const offlineCommands = createTeleportCommands(
      {} as StreamplaceAgent,
      "did:plc:offline",
      () => null,
    );

    const offline = await handleSlashCommand("/tp invalid", offlineCommands);
    const live = await handleSlashCommand("/tp invalid", commands);

    expect(offline.errorData?.code).toBe("not-live");
    expect(live.errorData?.code).toBe("handle-format");
  });

  it.each(["5seconds", "5.5", "Infinity"])(
    "rejects a countdown that is not a whole number: %s",
    async (countdown) => {
      register({});

      const result = await handleSlashCommand(
        `/teleport @target.example ${countdown}`,
        commands,
      );

      expect(result.errorData).toEqual({
        type: "teleport",
        code: "countdown-number",
      });
    },
  );

  it("returns a stable code for invalid handles", async () => {
    register({});

    const result = await handleSlashCommand("/teleport invalid", commands);

    expect(result.errorData).toEqual({
      type: "teleport",
      code: "handle-format",
    });
  });

  it("returns a stable code and handle when resolution fails", async () => {
    register({
      resolveHandle: vi.fn().mockRejectedValue(new Error("network error")),
    });

    const result = await handleSlashCommand(
      "/teleport @missing.example",
      commands,
    );

    expect(result.errorData).toEqual({
      type: "teleport",
      code: "resolve-handle",
      params: { handle: "missing.example" },
    });
  });

  it("accepts a DID from the live-stream picker", async () => {
    const create = vi.fn().mockResolvedValue({
      uri: "at://did:plc:streamer/place.stream.live.teleport/3mteleport",
    });
    register({ client: { create } } as unknown as StreamplaceAgent);

    await handleSlashCommand("/teleport did:plc:target 10", commands);

    expect(create.mock.calls[0]?.[1]).toMatchObject({
      streamer: "did:plc:target",
    });
  });
});

describe("createTeleport", () => {
  it("targets a streamer by DID without resolving a handle", async () => {
    const resolveHandle = vi.fn();
    const create = vi.fn().mockResolvedValue({ uri: "at://teleport" });
    const agent = { resolveHandle, client: { create } };

    const result = await createTeleport(
      agent as unknown as StreamplaceAgent,
      "did:plc:streamer",
      "did:plc:target",
      10,
    );

    expect(result).toEqual({ success: true });
    expect(resolveHandle).not.toHaveBeenCalled();
    expect(create.mock.calls[0][1]).toMatchObject({
      streamer: "did:plc:target",
    });
  });
});
