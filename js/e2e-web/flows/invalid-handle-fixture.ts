import { expect, request } from "@playwright/test";

// Move the real harness account to a .test handle: its DID document changes,
// but public handle resolution cannot verify it. Keep the same DID/stream and
// restore its original handle even if the browser assertion fails.
export async function withInvalidHandle(run: () => Promise<void>) {
  const pds = await request.newContext({ ignoreHTTPSErrors: true });
  const did = process.env.ACCOUNT_DID!;
  const node = process.env.SERVER_URL!;
  const pdsUrl = process.env.PDS_HTTP_URL!;
  const session = await pds.post(
    `${pdsUrl}/xrpc/com.atproto.server.createSession`,
    { data: { identifier: did, password: process.env.ACCOUNT_PASSWORD! } },
  );
  expect(session.ok(), await session.text()).toBe(true);
  const { accessJwt } = await session.json();
  const updateHandle = async (handle: string) => {
    const response = await pds.post(
      `${pdsUrl}/xrpc/com.atproto.identity.updateHandle`,
      { headers: { Authorization: `Bearer ${accessJwt}` }, data: { handle } },
    );
    expect(response.ok(), await response.text()).toBe(true);
    const refreshed = await pds.post(
      `${node}/xrpc/com.atproto.identity.refreshIdentity`,
      { data: { identifier: did } },
    );
    expect(refreshed.ok(), await refreshed.text()).toBe(true);
    return refreshed.json();
  };
  // Live feeds cache identities for 30 seconds. Wait for the real response
  // after each transition, not a fixed sleep or a mock.
  const waitForFeedHandle = async (handle: string) => {
    await expect
      .poll(
        async () => {
          const response = await pds.get(
            `${node}/xrpc/place.stream.live.getLiveUsers?limit=50`,
          );
          expect(response.ok()).toBe(true);
          const body = await response.json();
          return body.streams?.find(
            (stream: { author: { did: string; handle: string } }) =>
              stream.author.did === did,
          )?.author.handle;
        },
        { timeout: 45_000, intervals: [1000] },
      )
      .toBe(handle);
  };
  try {
    const identity = await updateHandle(
      `invalid-${Date.now().toString(36)}.test`,
    );
    expect(identity.handle).toBe("handle.invalid");
    await waitForFeedHandle(identity.handle);
    await run();
  } finally {
    try {
      const identity = await updateHandle(process.env.ACCOUNT_HANDLE!);
      // A plain-HTTP .test harness cannot verify either handle.
      if (process.env.SERVER_HTTPS_URL) {
        expect(identity.handle).toBe(process.env.ACCOUNT_HANDLE);
      }
      await waitForFeedHandle(identity.handle);
    } finally {
      await pds.dispose();
    }
  }
}
