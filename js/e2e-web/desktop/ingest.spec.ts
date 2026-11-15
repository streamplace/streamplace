import { expect, test } from "./fixture";
import { loginThroughSystemBrowser } from "./login";

// The Desktop harness has no media ingest; verify authorization and the real PDS record.
test("internal ingest uses the logged-in account without an OBS key", async ({
  desktop,
  request,
}) => {
  const configuration = "/api/ingest/client-configuration";
  const data = { authentication: "streamplace-desktop" };
  const recordsURL = new URL(
    "/xrpc/com.atproto.repo.listRecords",
    process.env.PDS_HTTPS_URL!,
  );
  recordsURL.search = new URLSearchParams({
    repo: process.env.ACCOUNT_DID!,
    collection: "place.stream.key",
    limit: "100",
  }).toString();
  const desktopKeys = async () => {
    const response = await request.get(recordsURL.toString(), {
      ignoreHTTPSErrors: true,
    });
    expect(response.ok()).toBe(true);
    const body = await response.json();
    return body.records.filter(
      (record: { value: { createdBy?: string } }) =>
        record.value.createdBy === "Streamplace Desktop",
    );
  };
  const before = (await desktopKeys()).length;
  const publicResponse = await request.post(desktop.nodeURL + configuration, {
    data,
  });
  expect(publicResponse.status()).toBe(404);
  const loggedOut = await request.post(desktop.internalURL + configuration, {
    data,
  });
  expect(loggedOut.status()).toBe(401);
  expect(await loggedOut.text()).toContain("log in to Streamplace Desktop");
  await loginThroughSystemBrowser(desktop);
  await expect
    .poll(
      async () => {
        const response = await request.post(
          desktop.internalURL + configuration,
          {
            data,
          },
        );
        if (!response.ok()) return response.status();
        return response.json();
      },
      { timeout: 60_000 },
    )
    .toMatchObject({
      status: { result: "success" },
      ingest_endpoints: [
        {
          protocol: "FMP4",
          url_template: `${desktop.internalURL}/api/ingest/fmp4/{stream_key}`,
        },
      ],
    });
  await expect.poll(async () => (await desktopKeys()).length).toBe(before + 1);
  for (const path of [configuration, "/api/ingest/fmp4/streamplace-desktop"]) {
    const response = await request.post(desktop.internalURL + path, {
      data,
      headers: { "Sec-Fetch-Site": "cross-site" },
    });
    expect(response.status()).toBe(403);
  }
  await desktop.restart();
  const response = await request.post(desktop.internalURL + configuration, {
    data,
  });
  expect(response.ok()).toBe(true);
  expect((await response.json()).status.result).toBe("success");
  expect((await desktopKeys()).length).toBe(before + 1);
});
