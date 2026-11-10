import { expect, type Page } from "@playwright/test";

// Sends a chat message on the harness stream as the page's signed-in user and
// waits for the record to land on their PDS: a write the node makes through
// the user's upstream (DPoP-bound) OAuth session.
export async function sendChatMessage(
  page: Page,
  url: string,
  message: string,
) {
  await page.goto(`${url}/`);
  await page.getByTestId("home-stream-card").first().click();
  // chat needs the streamer's profile, which arrives over the stream's
  // websocket along with this system message
  await expect(
    page.getByText("Now streaming - e2e test stream").first(),
  ).toBeVisible({ timeout: 30_000 });
  const chatInput = page.getByPlaceholder("Type a message...");
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
    await expect(page.getByText(message).first()).toBeVisible({
      timeout: 10_000,
    });
  }).toPass({ timeout: 45_000 });
  await expect(chatInput).toHaveValue("");
  const response = await messageCreateResponse;
  expect(
    response.ok(),
    `chat record creation returned ${response.status()}`,
  ).toBe(true);
}
