import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const HTTPS_URL = process.env.SERVER_HTTPS_URL;
test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

for (const timezoneId of ["America/Los_Angeles", "Asia/Kathmandu", "UTC"]) {
  test.describe(timezoneId, () => {
    test.use({ timezoneId });

    test("chat preserves the sending device's UTC offset", async ({ page }) => {
      await loginThroughPds(page);
      // Both bundles have their own send hook. Switching to the web beta
      // should preserve the session, but each hook must stamp local time.
      for (const frontend of ["app", "web"]) {
        if (frontend === "web") {
          await page
            .context()
            .addCookies([{ name: "sp_web_beta", value: "1", url: HTTPS_URL! }]);
          await page.goto(
            `${HTTPS_URL}/chat-popout/${process.env.ACCOUNT_DID}`,
          );
        } else {
          await page.goto(`${HTTPS_URL}/`);
          await page.getByTestId("home-stream-card").first().click();
        }
        await expect(
          page.getByText("Now streaming - e2e test stream").first(),
        ).toBeVisible();

        const message = `timezone check ${frontend} ${timezoneId} ${Date.now()}`;
        const responsePromise = page.waitForResponse((response) => {
          const request = response.request();
          return (
            request.method() === "POST" &&
            new URL(response.url()).pathname ===
              "/xrpc/com.atproto.repo.createRecord" &&
            request.postData()?.includes(message) === true
          );
        });
        const before = Date.now();
        const input =
          frontend === "web"
            ? page.locator('[contenteditable="true"]')
            : page.getByPlaceholder("Type a message...");
        await input.fill(message);
        await input.press("Enter");
        const response = await responsePromise;
        expect(response.ok()).toBe(true);
        const { createdAt } = response.request().postDataJSON().record;
        expect(typeof createdAt).toBe("string");
        expect(Date.parse(createdAt)).toBeGreaterThanOrEqual(before);
        expect(Date.parse(createdAt)).toBeLessThanOrEqual(Date.now());
        const offset = await page.evaluate(
          (timestamp) => new Date(timestamp).getTimezoneOffset(),
          createdAt,
        );
        const magnitude = Math.abs(offset);
        const expectedOffset = `${offset > 0 ? "-" : "+"}${String(
          Math.floor(magnitude / 60),
        ).padStart(2, "0")}:${String(magnitude % 60).padStart(2, "0")}`;
        expect(createdAt.endsWith(expectedOffset)).toBe(true);

        // Only the indexed history can restore a message after a reload;
        // the optimistic local echo alone must not satisfy this test.
        await expect(page.getByText(message).first()).toBeVisible();
        await page.reload();
        await expect(page.getByText(message).first()).toBeVisible();
      }
    });
  });
}
