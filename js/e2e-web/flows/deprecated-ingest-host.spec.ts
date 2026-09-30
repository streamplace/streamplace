import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

// A streamer whose encoder still points at an old hostname for the node is
// warned on their live dashboard. The harness's second account streams RTMP
// to the node through the hostname it treats as deprecated (see
// e2eDeprecatedIngestHost in pkg/cmd/e2e.go); the node spots it in the RTMP
// tcUrl and reports it over the stream's websocket. Mobile:
// .maestro/logged-in/deprecated-ingest-host.yaml.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;
const HANDLE = process.env.DEPRECATED_HOST_ACCOUNT_HANDLE;
const PASSWORD = process.env.DEPRECATED_HOST_ACCOUNT_PASSWORD;

test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

test("deprecated-ingest-host: the dashboard says to change the server", async ({
  page,
}) => {
  await loginThroughPds(page, { handle: HANDLE!, password: PASSWORD! });

  await page.goto(`${HTTPS_URL}/live`);
  await expect(page.getByText("Update your stream server")).toBeVisible({
    timeout: 60_000,
  });
  // names the old address, and the one to use instead: this node's RTMP
  // ingest
  await expect(
    page.getByText(
      /streaming to localhost, an old address.*to rtmp:\/\/\S+\/live\./,
    ),
  ).toBeVisible();

  // acknowledged, it stays counted in the dashboard's issues
  await page.getByText("Acknowledge").click();
  await expect(page.getByText("Update your stream server")).toBeHidden();
  await expect(page.getByText(/^\d+ Issues?$/).first()).toBeVisible();
});
