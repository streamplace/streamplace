import { expect, test } from "@playwright/test";

// Server-side handle discovery; there is no iOS or Android UI surface.
// The peer advertises the harness account's PLC DID via SP_ATPROTO_DID,
// without changing its own did:web identity.
test("atproto DID override does not replace the node's repo identity", async ({
  request,
}) => {
  const peer = process.env.SERVER2_URL!;
  const response = await request.get(`${peer}/.well-known/atproto-did`);
  expect(response.status()).toBe(200);
  expect(await response.text()).toBe(process.env.ACCOUNT_DID);

  const document = await request.get(`${peer}/.well-known/did.json`);
  expect(document.status()).toBe(200);
  expect((await document.json()).id).toBe(`did:web:${new URL(peer).host}`);
});
