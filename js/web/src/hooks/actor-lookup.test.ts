import { describe, expect, it } from "vitest";
import { resolvePublicActor } from "./actor-lookup";

describe("resolvePublicActor", () => {
  it("returns a resolved handle and DID", async () => {
    const result = await resolvePublicActor("natalie.sh", async () =>
      Response.json({
        did: "did:plc:natalie",
        handle: "natalie.sh",
      }),
    );

    expect(result).toEqual({
      status: "found",
      actor: { did: "did:plc:natalie", handle: "natalie.sh" },
    });
  });

  it("distinguishes a missing profile from a temporary failure", async () => {
    const result = await resolvePublicActor(
      "missing.example",
      async () =>
        new Response(
          JSON.stringify({
            error: "InvalidRequest",
            message: "Profile not found",
          }),
          { status: 400 },
        ),
    );

    expect(result).toEqual({ status: "not-found" });
  });

  it("throws on service failures so the UI can offer a retry", async () => {
    await expect(
      resolvePublicActor(
        "natalie.sh",
        async () => new Response("unavailable", { status: 503 }),
      ),
    ).rejects.toThrow("Profile lookup failed (503)");
  });
});
