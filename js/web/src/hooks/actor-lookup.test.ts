import { describe, expect, it } from "vitest";
import { resolveActor } from "./actor-lookup";

const NODE = "https://node.example";

describe("resolveActor", () => {
  it("recognizes an indexed streamer even when the public AppView has no profile", async () => {
    const result = await resolveActor("did:plc:streamer", NODE, async (url) =>
      String(url).startsWith(NODE)
        ? Response.json({
            author: { did: "did:plc:streamer", handle: "streamer.example" },
          })
        : Response.json({ message: "Profile not found" }, { status: 400 }),
    );
    expect(result).toEqual({
      status: "found",
      actor: { did: "did:plc:streamer", handle: "streamer.example" },
    });
  });

  it("recognizes an account without an indexed stream through the existing public profile lookup", async () => {
    const result = await resolveActor("viewer.example", NODE, async (url) =>
      String(url).startsWith(NODE)
        ? new Response(null, { status: 404 })
        : Response.json({ did: "did:plc:viewer", handle: "viewer.example" }),
    );
    expect(result).toEqual({
      status: "found",
      actor: { did: "did:plc:viewer", handle: "viewer.example" },
    });
  });

  it("reports not found only when neither lookup recognizes the actor", async () => {
    const result = await resolveActor("missing.example", NODE, async (url) =>
      String(url).startsWith(NODE)
        ? new Response(null, { status: 404 })
        : Response.json({ message: "Profile not found" }, { status: 400 }),
    );
    expect(result).toEqual({ status: "not-found" });
  });

  it("keeps service failures distinct from a missing account", async () => {
    await expect(
      resolveActor(
        "streamer.example",
        NODE,
        async () => new Response("unavailable", { status: 503 }),
      ),
    ).rejects.toThrow("Profile lookup failed (503)");
  });
});
