import { expect, test } from "@playwright/test";

// Mirror of .maestro/logged-out/follow-control.yaml: the follow action shows a
// user-plus glyph, and signed out it routes to the login gate instead of
// mutating follow state.
test("follow-control: signed-out follow shows user-plus and opens login", async ({
  page,
}) => {
  await page.goto("/");

  const card = page.getByTestId("home-stream-card").first();
  await expect(card).toBeVisible({ timeout: 30_000 });
  await card.click();

  const follow = page.getByTestId("follow-button");
  await expect(follow).toBeVisible({ timeout: 30_000 });
  await expect(follow).toHaveAccessibleName("Follow");
  await expect(follow.getByTestId("follow-icon-user-plus")).toBeVisible();

  await follow.click();
  // Sidebar chrome already says "Log in"; this copy only exists in the modal.
  await expect(
    page.getByText("Sign in using your handle on the AT Protocol", {
      exact: true,
    }),
  ).toBeVisible({ timeout: 15_000 });
});
