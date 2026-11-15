import { describe, expect, it } from "vitest";
import { resolveActor } from "./use-actor-lookup";

const respond = (response: Response) => async () => response;

describe("resolveActor", () => {
  it("returns the node's DID and handle for an existing repo", async () => {
    let requested = "";
    const result = await resolveActor(
      "did:plc:natalie",
      "https://node.example/",
      async (input) => {
        requested = String(input);
        return Response.json({ did: "did:plc:natalie", handle: "natalie.sh" });
      },
    );

    expect(requested).toBe(
      "https://node.example/xrpc/com.atproto.repo.describeRepo?repo=did%3Aplc%3Anatalie",
    );
    expect(result).toEqual({
      status: "found",
      actor: { did: "did:plc:natalie", handle: "natalie.sh" },
    });
  });

  it.each([
    ["node 404 message", 404, { message: "RepoNotFound" }],
    ["node 404 XRPC error", 404, { error: "RepoNotFound" }],
    ["upstream PDS 400", 400, { error: "RepoNotFound", message: "no repo" }],
  ])("treats %s as a missing account", async (_, status, body) => {
    const result = await resolveActor(
      "missing.example",
      "https://node.example",
      respond(Response.json(body, { status })),
    );

    expect(result).toEqual({ status: "not-found" });
  });

  it.each([
    ["an outage", new Response("unavailable", { status: 503 })],
    ["an unsupported endpoint", new Response("Not Found", { status: 404 })],
    [
      "an unrelated bad request",
      Response.json({ error: "InvalidRequest" }, { status: 400 }),
    ],
    [
      "RepoNotFound on a server error",
      Response.json({ error: "RepoNotFound" }, { status: 500 }),
    ],
    ["invalid success JSON", new Response("<html>", { status: 200 })],
    ["a malformed success body", Response.json({ handle: "natalie.sh" })],
  ])("throws on %s so the UI can offer a retry", async (_, response) => {
    await expect(
      resolveActor("natalie.sh", "https://node.example", respond(response)),
    ).rejects.toThrow();
  });
});
