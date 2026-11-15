import { expect, test } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";

// Web-only geometry: native login typography and positioning are unchanged.
test("login typeahead matches the input's typography and baseline", async ({
  page,
}, testInfo) => {
  const url = process.env.SERVER_HTTPS_URL;
  test.skip(!url, "The login form requires the harness's HTTPS metadata.");
  await page.route(
    "https://public.api.bsky.app/xrpc/app.bsky.actor.searchActorsTypeahead*",
    (route) =>
      route.fulfill({
        headers: { "Access-Control-Allow-Origin": "*" },
        json: {
          actors: [
            { did: process.env.ACCOUNT_DID, handle: "alice.example.test" },
          ],
        },
      }),
  );
  await pointAppAtNode(page, url!);
  await page.goto(`${url}/login`);
  const input = page.getByTestId("login-handle");
  const completion = page.getByText("alice.example.test", { exact: true });

  for (const width of [1280, 390]) {
    await page.setViewportSize({ width, height: 800 });
    for (const prefix of ["ali", "alice.ex"]) {
      await input.fill(prefix);
      await expect(completion).toBeVisible();
      const geometry = await completion.evaluate((element) => {
        const input = document.querySelector<HTMLInputElement>(
          '[data-testid="login-handle"]',
        )!;
        const inputStyle = getComputedStyle(input);
        const style = getComputedStyle(element);
        const prefix = element.firstElementChild!;
        const prefixStyle = getComputedStyle(prefix);
        const inputBox = input.getBoundingClientRect();
        const box = element.getBoundingClientRect();
        const prefixBox = prefix.getBoundingClientRect();
        return {
          inputFont: inputStyle.font,
          completionFont: style.font,
          prefixFont: prefixStyle.font,
          verticalOffset:
            box.top + box.height / 2 - (inputBox.top + inputBox.height / 2),
          horizontalOffset:
            prefixBox.left -
            inputBox.left -
            parseFloat(inputStyle.borderLeftWidth) -
            parseFloat(inputStyle.paddingLeft),
        };
      });
      expect(geometry.completionFont).toBe(geometry.inputFont);
      expect(geometry.prefixFont).toBe(geometry.inputFont);
      expect(Math.abs(geometry.verticalOffset)).toBeLessThan(1);
      expect(Math.abs(geometry.horizontalOffset)).toBeLessThan(1);
    }
    await testInfo.attach(`typeahead-${width}`, {
      body: await input.locator("../..").screenshot(),
      contentType: "image/png",
    });
    await input.press("Tab");
    await expect(input).toHaveValue("alice.example.test");
    await expect(completion).toHaveCount(0);
  }
});
