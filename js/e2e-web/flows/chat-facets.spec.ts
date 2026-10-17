import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

// A message's link facets have to be typed with the place lexicon, since
// that is what place.stream.richtext.facet requires and what anything
// validating a chat record holds them to. The mobile counterpart is
// .maestro/logged-in/chat-facets.yaml. See flows/login.ts for the OAuth
// machinery; it needs the harness's HTTPS mode.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;

test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

test("chat-facets: send a link and check the facets it writes", async ({
  page,
}) => {
  await loginThroughPds(page);

  await page.goto(`${HTTPS_URL}/`);
  await page.getByTestId("home-stream-card").first().click();
  // chat needs the streamer's profile, which arrives over the stream's
  // websocket along with this system message
  await expect(
    page.getByText("Now streaming - e2e test stream").first(),
  ).toBeVisible({ timeout: 30_000 });

  const chatInput = page.getByPlaceholder("Type a message...");
  const marker = `link facet check ${Date.now()}`;
  const link = "https://dist.town/release.software";
  const message = `${marker}: read the ${link} docs`;
  const messageCreateResponse = page.waitForResponse((response) => {
    const request = response.request();
    return (
      request.method() === "POST" &&
      new URL(response.url()).pathname ===
        "/xrpc/com.atproto.repo.createRecord" &&
      request.postData()?.includes(message) === true
    );
  });
  // a send the page wasn't ready for clears the box and is dropped; resend
  await expect(async () => {
    await chatInput.fill(message);
    await chatInput.press("Enter");
    await expect(page.getByText(marker).first()).toBeVisible({
      timeout: 10_000,
    });
  }).toPass({ timeout: 45_000 });

  const response = await messageCreateResponse;
  expect(
    response.ok(),
    `chat record creation returned ${response.status()}`,
  ).toBe(true);
  // detectFacets leaves link facets untyped and types mention facets as
  // app.bsky's, and a record carrying either is rejected by anything
  // validating it against place.stream.richtext.facet.
  expect(response.request().postDataJSON()).toMatchObject({
    record: {
      text: message,
      facets: [
        {
          $type: "place.stream.richtext.facet",
          features: [{ $type: "app.bsky.richtext.facet#link", uri: link }],
        },
      ],
    },
  });

  // The box shows sends optimistically, so reload: only the node's chat
  // history can put the message back, and it only has what the firehose
  // brought back from the PDS.
  await page.reload();
  await expect(page.getByText(marker).first()).toBeVisible({
    timeout: 30_000,
  });
});
