import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

// Shared native counterpart: .maestro/logged-out/profile-pronouns.yaml.
// Profile records, indexing, stream websocket, and chat stay real. Only public
// AppView discovery is substituted: it cannot know the local harness account.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;
const DID = process.env.ACCOUNT_DID!;
const HANDLE = process.env.ACCOUNT_HANDLE!;
test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

for (const frontend of ["app", "web"]) {
  test(`${frontend}: Bluesky pronouns appear in stream identity and chat profiles`, async ({
    page,
    request,
  }, testInfo) => {
    await loginThroughPds(page);
    if (frontend === "web") {
      await page
        .context()
        .addCookies([{ name: "sp_web_beta", value: "1", url: HTTPS_URL! }]);
    }
    const pds = process.env.PDS_HTTP_URL!;
    const session = await request.post(
      `${pds}/xrpc/com.atproto.server.createSession`,
      { data: { identifier: DID, password: process.env.ACCOUNT_PASSWORD! } },
    );
    expect(session.ok(), await session.text()).toBe(true);
    const { accessJwt } = await session.json();
    const headers = { Authorization: `Bearer ${accessJwt}` };
    const recordUrl = `${pds}/xrpc/com.atproto.repo.getRecord?repo=${DID}&collection=app.bsky.actor.profile&rkey=self`;
    const original = await request.get(recordUrl);
    const saved = await original.json();
    if (!original.ok()) expect(saved.error).toBe("RecordNotFound");
    const base = original.ok()
      ? saved.value
      : { $type: "app.bsky.actor.profile" };
    let pronouns: string | undefined;
    const putProfile = async (value: Record<string, unknown>) => {
      const response = await request.post(
        `${pds}/xrpc/com.atproto.repo.putRecord`,
        {
          headers,
          data: {
            repo: DID,
            collection: "app.bsky.actor.profile",
            rkey: "self",
            record: value,
          },
        },
      );
      expect(response.ok(), await response.text()).toBe(true);
    };
    await page.route(
      "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfiles*",
      (route) =>
        route.fulfill({
          contentType: "application/json",
          headers: { "access-control-allow-origin": "*" },
          body: JSON.stringify({
            profiles: [{ did: DID, handle: HANDLE, pronouns }],
          }),
        }),
    );
    await page.route(
      "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?*",
      (route) =>
        route.fulfill({
          headers: { "access-control-allow-origin": "*" },
          json: { did: DID, handle: HANDLE },
        }),
    );
    // Observe the real initial profile burst so absence assertions cannot pass
    // merely because indexing or the websocket has not loaded the profile yet.
    let indexedPronouns: string | undefined | null = null;
    page.on("websocket", (socket) => {
      if (!socket.url().includes("/api/websocket/")) return;
      socket.on("framereceived", ({ payload }) => {
        const messages = JSON.parse(payload.toString());
        for (const message of Array.isArray(messages) ? messages : [messages]) {
          if (
            message.$type === "app.bsky.actor.defs#profileViewBasic" &&
            message.did === DID
          ) {
            indexedPronouns = message.pronouns;
          }
        }
      });
    });
    const message = `pronouns profile ${frontend} ${Date.now()}`;
    try {
      for (pronouns of [" xe/xem · 🦋 ", "   ", undefined]) {
        const value = { ...base };
        delete value.pronouns;
        if (pronouns !== undefined) value.pronouns = pronouns;
        await putProfile(value);
        await expect(async () => {
          indexedPronouns = null;
          await page.goto(`${HTTPS_URL}/${DID}`);
          await expect
            .poll(() => indexedPronouns, { timeout: 5000 })
            .toBe(pronouns);
        }).toPass({ timeout: 45_000 });
        const streamPronouns = page.getByTestId("stream-pronouns");
        if (pronouns?.trim()) {
          await expect(streamPronouns).toHaveText(pronouns.trim());
          await page.screenshot({
            path: testInfo.outputPath("stream-pronouns.png"),
          });
        } else {
          await expect(streamPronouns).toHaveCount(0);
        }
        const input =
          frontend === "app"
            ? page.getByPlaceholder("Type a message...")
            : page.locator('[contenteditable="true"]:visible');
        // Send once; subsequent reloads must load that message from history.
        if (pronouns?.trim()) {
          await expect(input).toBeVisible();
          await input.fill(message);
          await input.press("Enter");
        }
        const visibleMessage = page
          .getByText(message, { exact: true })
          .filter({ visible: true })
          .first();
        await expect(visibleMessage).toBeVisible();
        if (frontend === "app") {
          await page
            .getByTestId(`chat-profile-trigger-${HANDLE}`)
            .first()
            .click();
        } else {
          await page
            .getByRole("button", { name: HANDLE, exact: true })
            .first()
            .click();
        }
        await expect(
          frontend === "app"
            ? page.getByTestId("chat-profile-card")
            : page.getByRole("link", { name: /View profile on/ }),
        ).toBeVisible();
        const cardPronouns = page.getByTestId("chat-profile-pronouns");
        if (pronouns?.trim()) {
          await expect(cardPronouns).toHaveText(pronouns.trim());
          await page.screenshot({
            path: testInfo.outputPath("chat-profile-pronouns.png"),
          });
        } else {
          await expect(cardPronouns).toHaveCount(0);
        }
      }
    } finally {
      // Profile deletions are not indexed by the node; restore an empty
      // profile when none existed so failed assertions cannot leave pronouns.
      await putProfile(base);
    }
  });
}
