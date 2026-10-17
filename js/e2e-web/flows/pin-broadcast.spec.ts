import { expect, test, type Page } from "@playwright/test";

const pdsUrl = process.env.PDS_HTTPS_URL;
test.skip(!pdsUrl, "needs the harness's HTTPS PDS");

async function pds(page: Page, method: string, input: object, token?: string) {
  return page.evaluate(
    async ({ url, method, input, token }) => {
      const response = await fetch(`${url}/xrpc/${method}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify(input),
      });
      if (!response.ok)
        throw new Error(
          `${method}: ${response.status} ${await response.text()}`,
        );
      return response.json();
    },
    { url: pdsUrl!, method, input, token },
  );
}

for (const frontend of ["app", "web"]) {
  test(`${frontend}: pins survive chapters and expire for connected and returning viewers`, async ({
    page,
  }) => {
    if (frontend === "web") {
      await page
        .context()
        .addCookies([
          { name: "sp_web_beta", value: "1", url: process.env.SERVER_URL! },
        ]);
    }
    await page.goto("/");
    const suffix = process.env.ACCOUNT_HANDLE!.split(".").slice(1).join(".");
    const account = await pds(page, "com.atproto.server.createAccount", {
      handle: `pin-${Date.now()}.${suffix}`,
      email: `pin-${Date.now()}@example.invalid`,
      password: "local-pin-test-password",
    });
    const write = (collection: string, record: object) =>
      pds(
        page,
        "com.atproto.repo.createRecord",
        { repo: account.did, collection, record },
        account.accessJwt,
      );
    const record = {
      $type: "place.stream.livestream",
      title: "pin test broadcast",
      createdAt: new Date().toISOString(),
    };
    const first = await write("place.stream.livestream", record);
    const message = await write("place.stream.chat.message", {
      $type: "place.stream.chat.message",
      text: "pin across chapters",
      streamer: account.did,
      createdAt: new Date().toISOString(),
    });
    await write("place.stream.chat.pinnedRecord", {
      $type: "place.stream.chat.pinnedRecord",
      pinnedMessage: message.uri,
      livestream: first.uri,
      createdAt: new Date().toISOString(),
    });

    await page.goto(`/${account.did}`);
    const pinned = page.getByTestId("pinned-message");
    await expect(pinned).toContainText("pin across chapters");

    const chapterRecord = {
      ...record,
      title: "pin test next chapter",
      broadcast: first.uri,
      createdAt: new Date().toISOString(),
    };
    const chapter = await write("place.stream.livestream", chapterRecord);
    await expect(page.getByText("pin test next chapter").first()).toBeVisible();
    await expect(pinned).toContainText("pin across chapters");
    await page.reload();
    await expect(pinned).toContainText("pin across chapters");

    await pds(
      page,
      "com.atproto.repo.putRecord",
      {
        repo: account.did,
        collection: "place.stream.livestream",
        rkey: chapter.uri.split("/").pop(),
        record: { ...chapterRecord, endedAt: new Date().toISOString() },
      },
      account.accessJwt,
    );
    await expect(pinned).toHaveCount(0);
    await page.reload();
    await expect(page.getByText("pin test next chapter").first()).toBeVisible();
    await expect(pinned).toHaveCount(0);

    await write("place.stream.livestream", {
      ...record,
      title: "pin test separate broadcast",
      createdAt: new Date().toISOString(),
    });
    await expect(
      page.getByText("pin test separate broadcast").first(),
    ).toBeVisible();
    await expect(pinned).toHaveCount(0);
    await page.reload();
    await expect(
      page.getByText("pin test separate broadcast").first(),
    ).toBeVisible();
    await expect(pinned).toHaveCount(0);
  });
}
