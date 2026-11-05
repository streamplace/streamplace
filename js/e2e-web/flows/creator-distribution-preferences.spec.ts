import { expect, request, test, type Page } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
const pdsUrl = process.env.PDS_HTTPS_URL;
const repo = process.env.ACCOUNT_DID;
const collection = "place.stream.metadata.configuration";
test.skip(
  !url || !pdsUrl || !repo,
  "harness started without its HTTPS account",
);

async function metadataAccount() {
  const pds = await request.newContext({ ignoreHTTPSErrors: true });
  const session = await pds.post(
    `${pdsUrl}/xrpc/com.atproto.server.createSession`,
    {
      data: {
        identifier: process.env.ACCOUNT_HANDLE,
        password: process.env.ACCOUNT_PASSWORD,
      },
    },
  );
  expect(session.ok()).toBe(true);
  const headers = {
    Authorization: `Bearer ${(await session.json()).accessJwt}`,
  };
  const existing = await pds.get(`${pdsUrl}/xrpc/com.atproto.repo.getRecord`, {
    params: { repo: repo!, collection, rkey: "self" },
  });
  const original = existing.ok() ? (await existing.json()).value : undefined;
  if (!existing.ok())
    expect((await existing.json()).error).toBe("RecordNotFound");
  const put = async (record: object) => {
    const response = await pds.post(
      `${pdsUrl}/xrpc/com.atproto.repo.putRecord`,
      {
        headers,
        data: { repo, collection, rkey: "self", record },
      },
    );
    expect(response.ok()).toBe(true);
  };
  return {
    put,
    async get() {
      const response = await pds.get(
        `${pdsUrl}/xrpc/com.atproto.repo.getRecord`,
        {
          params: { repo: repo!, collection, rkey: "self" },
        },
      );
      expect(response.ok()).toBe(true);
      return (await response.json()).value;
    },
    async restore() {
      if (original) await put(original);
      else {
        const response = await pds.post(
          `${pdsUrl}/xrpc/com.atproto.repo.deleteRecord`,
          {
            headers,
            data: { repo, collection, rkey: "self" },
          },
        );
        expect(response.ok()).toBe(true);
      }
      await pds.dispose();
    },
  };
}

async function saveMetadata(page: Page) {
  const saved = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      /\/xrpc\/com\.atproto\.repo\.(createRecord|putRecord)$/.test(
        response.url(),
      ) &&
      response.request().postDataJSON()?.collection === collection,
  );
  await page
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  expect((await saved).ok()).toBe(true);
}

test("distribution toggles preserve edited values, including explicit clears", async ({
  page,
}) => {
  const account = await metadataAccount();
  try {
    await account.put({
      $type: collection,
      contentRights: { copyrightNotice: "Keep these rights" },
      distributionPolicy: {
        allowedBroadcasters: ["did:web:original.example"],
        deleteAfter: 1800,
        allowGenAiTraining: true,
      },
    });
    await loginThroughPds(page);
    await page.goto(`${url}/dashboard/stream`);
    const broadcasters = page
      .getByText("Allowed Broadcasters", { exact: true })
      .locator("..")
      .locator("textarea");
    const duration = page
      .getByText("Delete After", { exact: true })
      .locator("..")
      .locator("input");
    await expect(broadcasters).toHaveValue("did:web:original.example");
    await expect(duration).toHaveValue("1800");
    const everyone = page.getByRole("checkbox", {
      name: "Allow everyone to distribute your content",
    });
    const archive = page.getByRole("checkbox", {
      name: "Allow everyone to archive your content",
    });
    const ai = page.getByRole("checkbox", {
      name: "Mark your content as available for generative AI training",
    });

    await broadcasters.fill("did:web:first.example\ndid:web:second.example");
    await duration.fill("7200");
    await everyone.check();
    await everyone.uncheck();
    await archive.check();
    await archive.uncheck();
    await expect(broadcasters).toHaveValue(
      "did:web:first.example\ndid:web:second.example",
    );
    await expect(duration).toHaveValue("7200");
    await saveMetadata(page);
    await expect(
      page.getByText(/content metadata (created|updated)/i),
    ).toBeVisible();
    await page.reload();
    await expect(broadcasters).toHaveValue(
      "did:web:first.example\ndid:web:second.example",
    );
    await expect(duration).toHaveValue("7200");
    await expect(ai).toBeChecked();
    await expect(
      page.getByPlaceholder("Enter your copyright notice..."),
    ).toHaveValue("Keep these rights");

    await broadcasters.fill("");
    await duration.fill("");
    await everyone.check();
    await everyone.uncheck();
    await archive.check();
    await archive.uncheck();
    await expect(broadcasters).toHaveValue("");
    await expect(duration).toHaveValue("");
    await saveMetadata(page);
    await page.reload();
    await expect(broadcasters).toHaveValue("");
    await expect(duration).toHaveValue("");
  } finally {
    await account.restore();
  }
});

test("saving a warning preserves different warning choices from another tab", async ({
  page,
}) => {
  const account = await metadataAccount();
  const futureWarning = "place.stream.metadata.contentWarnings#future";
  try {
    await account.put({
      $type: collection,
      contentWarnings: { warnings: [futureWarning, futureWarning] },
    });
    await loginThroughPds(page);
    await page.goto(`${url}/dashboard/stream`);
    const death = page.getByRole("checkbox", { name: "Death", exact: true });
    const language = page.getByRole("checkbox", {
      name: "Language",
      exact: true,
    });
    await expect(death).not.toBeChecked();
    const other = await page.context().newPage();
    await other.goto(`${url}/dashboard/stream`);
    const otherLanguage = other.getByRole("checkbox", {
      name: "Language",
      exact: true,
    });
    await expect(otherLanguage).not.toBeChecked();
    await death.check();
    await otherLanguage.check();
    await saveMetadata(other);
    await saveMetadata(page);
    await page.reload();
    await expect(death).toBeChecked();
    await expect(language).toBeChecked();
    expect(
      (await account.get()).contentWarnings.warnings.filter(
        (warning: string) => warning === futureWarning,
      ),
    ).toHaveLength(2);

    await other.reload();
    const drugUse = other.getByRole("checkbox", {
      name: "Drug Use",
      exact: true,
    });
    await expect(otherLanguage).toBeChecked();
    await page.goto(`${url}/dashboard`);
    const widget = page.getByRole("region", { name: "Stream Info widget" });
    await widget.getByRole("button", { name: "Edit content warnings" }).click();
    const widgetDeath = widget.getByRole("checkbox", {
      name: "Death",
      exact: true,
    });
    await expect(widgetDeath).toBeChecked();
    await widgetDeath.uncheck();
    await drugUse.check();
    await saveMetadata(other);
    await saveMetadata(page);
    await page.reload();
    await widget.getByRole("button", { name: "Edit content warnings" }).click();
    await expect(
      widget.getByRole("checkbox", { name: "Death", exact: true }),
    ).not.toBeChecked();
    await expect(
      widget.getByRole("checkbox", { name: "Language", exact: true }),
    ).toBeChecked();
    await expect(
      widget.getByRole("checkbox", { name: "Drug Use", exact: true }),
    ).toBeChecked();
  } finally {
    await account.restore();
  }
});

test("warning edits made during a save do not replay earlier choices", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const account = await metadataAccount();
  let release: (() => void) | undefined;
  try {
    await account.put({ $type: collection });
    await loginThroughPds(page);
    await page.goto(`${url}/dashboard/stream`);
    const death = page.getByRole("checkbox", { name: "Death", exact: true });
    const language = page.getByRole("checkbox", {
      name: "Language",
      exact: true,
    });
    await death.check();
    let intercepted!: () => void;
    const held = new Promise<void>((resolve) => (intercepted = resolve));
    const resume = new Promise<void>((resolve) => (release = resolve));
    await page.route("**/xrpc/com.atproto.repo.putRecord", async (route) => {
      if (route.request().postDataJSON()?.collection === collection) {
        intercepted();
        await resume;
      }
      await route.continue();
    });
    const firstSave = page.waitForResponse(
      (response) =>
        response.url().endsWith("/xrpc/com.atproto.repo.putRecord") &&
        response.request().postDataJSON()?.collection === collection,
    );
    await page
      .getByRole("button", { name: /save metadata|update metadata/i })
      .click();
    await held;
    await language.check();
    release!();
    expect((await firstSave).ok()).toBe(true);
    await page.unrouteAll({ behavior: "wait" });
    await expect(
      page.getByRole("button", { name: /save metadata|update metadata/i }),
    ).toBeEnabled();
    const other = await page.context().newPage();
    await other.goto(`${url}/dashboard/stream`);
    const otherDeath = other.getByRole("checkbox", {
      name: "Death",
      exact: true,
    });
    await expect(otherDeath).toBeChecked();
    await otherDeath.uncheck();
    await saveMetadata(other);
    await saveMetadata(page);
    await page.reload();
    await expect(death).not.toBeChecked();
    await expect(language).toBeChecked();
  } finally {
    release?.();
    await page.unrouteAll({ behavior: "wait" });
    await account.restore();
  }
});

test("saving one rights field preserves other rights changed in another tab", async ({
  page,
}) => {
  const account = await metadataAccount();
  try {
    await account.put({
      $type: collection,
      contentRights: {
        creator: "Keep the creator",
        copyrightYear: 2024,
        copyrightNotice: "Original notice",
        creditLine: "Keep the credit",
      },
    });
    await loginThroughPds(page);
    await page.goto(`${url}/dashboard/stream`);
    const year = page
      .getByText("Copyright Year", { exact: true })
      .locator("..")
      .locator("input");
    await expect(year).toHaveValue("2024");
    const other = await page.context().newPage();
    await other.goto(`${url}/dashboard/stream`);
    const notice = other.getByPlaceholder("Enter your copyright notice...");
    await expect(notice).toHaveValue("Original notice");
    await year.fill("2026");
    await notice.fill("Notice changed elsewhere");
    await saveMetadata(other);
    await saveMetadata(page);
    await page.reload();
    await expect(year).toHaveValue("2026");
    await expect(
      page.getByPlaceholder("Enter your copyright notice..."),
    ).toHaveValue("Notice changed elsewhere");
    const saved = await account.get();
    expect(saved.contentRights.creator).toBe("Keep the creator");
    expect(saved.contentRights.creditLine).toBe("Keep the credit");
  } finally {
    await account.restore();
  }
});

test("restricted choices can be edited after opening unrestricted settings", async ({
  page,
}) => {
  const account = await metadataAccount();
  try {
    await account.put({
      $type: collection,
      distributionPolicy: { allowedBroadcasters: ["*"], deleteAfter: -1 },
    });
    await loginThroughPds(page);
    await page.goto(`${url}/dashboard/stream`);
    const everyone = page.getByRole("checkbox", {
      name: "Allow everyone to distribute your content",
    });
    const archive = page.getByRole("checkbox", {
      name: "Allow everyone to archive your content",
    });
    await expect(everyone).toBeChecked();
    await expect(archive).toBeChecked();
    await everyone.uncheck();
    await archive.uncheck();
    const broadcasters = page
      .getByText("Allowed Broadcasters", { exact: true })
      .locator("..")
      .locator("textarea");
    const duration = page
      .getByText("Delete After", { exact: true })
      .locator("..")
      .locator("input");
    await expect(broadcasters).not.toHaveValue("");
    await expect(duration).toHaveValue("300");
    await broadcasters.fill("did:web:restricted.example");
    await duration.fill("900");
    await everyone.check();
    await everyone.uncheck();
    await archive.check();
    await archive.uncheck();
    await expect(broadcasters).toHaveValue("did:web:restricted.example");
    await expect(duration).toHaveValue("900");
    await saveMetadata(page);
    await page.reload();
    await expect(broadcasters).toHaveValue("did:web:restricted.example");
    await expect(duration).toHaveValue("900");
  } finally {
    await account.restore();
  }
});

test("a failed settings load does not present empty distribution choices as saved", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await expect(page.getByTestId("creator-stream-status")).toHaveText("Live");
  await page.context().setOffline(true);
  try {
    await page
      .getByTestId("creator-navigation")
      .getByRole("link", { name: "Stream Settings", exact: true })
      .click();
    await expect(
      page.getByText("Could not load distribution settings."),
    ).toBeVisible();
    await expect(
      page.getByRole("checkbox", {
        name: "Allow everyone to distribute your content",
      }),
    ).toBeDisabled();
    await expect(
      page
        .getByText("Delete After", { exact: true })
        .locator("..")
        .locator("input"),
    ).toBeDisabled();
    await page.context().setOffline(false);
    await page
      .getByRole("button", { name: "Retry distribution settings" })
      .click();
    await expect(
      page.getByText("Could not load distribution settings."),
    ).toHaveCount(0);
    await expect(
      page.getByRole("checkbox", {
        name: "Allow everyone to distribute your content",
      }),
    ).toBeEnabled();
  } finally {
    await page.context().setOffline(false);
  }
});

test("saving one distribution choice preserves other choices changed in another tab", async ({
  page,
}) => {
  const account = await metadataAccount();
  try {
    await account.put({
      $type: collection,
      distributionPolicy: {
        allowedBroadcasters: ["did:web:original.example"],
        deleteAfter: 1800,
        allowGenAiTraining: false,
      },
    });
    await loginThroughPds(page);
    await page.goto(`${url}/dashboard/stream`);
    const duration = page
      .getByText("Delete After", { exact: true })
      .locator("..")
      .locator("input");
    await expect(duration).toHaveValue("1800");
    const other = await page.context().newPage();
    await other.goto(`${url}/dashboard/stream`);
    const otherBroadcasters = other
      .getByText("Allowed Broadcasters", { exact: true })
      .locator("..")
      .locator("textarea");
    await expect(otherBroadcasters).toHaveValue("did:web:original.example");
    await duration.fill("7200");
    await otherBroadcasters.fill("did:web:changed-elsewhere.example");
    await other
      .getByRole("checkbox", {
        name: "Mark your content as available for generative AI training",
      })
      .check();
    await saveMetadata(other);
    await saveMetadata(page);
    await page.reload();
    await expect(duration).toHaveValue("7200");
    await expect(
      page
        .getByText("Allowed Broadcasters", { exact: true })
        .locator("..")
        .locator("textarea"),
    ).toHaveValue("did:web:changed-elsewhere.example");
    await expect(
      page.getByRole("checkbox", {
        name: "Mark your content as available for generative AI training",
      }),
    ).toBeChecked();
  } finally {
    await account.restore();
  }
});

test("a late settings load fills untouched choices without replacing a draft", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const account = await metadataAccount();
  let release: (() => void) | undefined;
  try {
    await account.put({
      $type: collection,
      distributionPolicy: {
        allowedBroadcasters: ["did:web:loaded-later.example"],
        deleteAfter: 1800,
        allowGenAiTraining: true,
      },
    });
    await loginThroughPds(page);
    let intercepted!: () => void;
    const held = new Promise<void>((resolve) => (intercepted = resolve));
    const resume = new Promise<void>((resolve) => (release = resolve));
    await page.route("**/xrpc/com.atproto.repo.getRecord?*", async (route) => {
      if (
        new URL(route.request().url()).searchParams.get("collection") ===
        collection
      ) {
        intercepted();
        await resume;
      }
      await route.continue();
    });
    await page.goto(`${url}/dashboard/stream`);
    await held;
    const everyone = page.getByRole("checkbox", {
      name: "Allow everyone to distribute your content",
    });
    await expect(everyone).toBeDisabled();
    const duration = page
      .getByText("Delete After", { exact: true })
      .locator("..")
      .locator("input");
    await duration.fill("7200");
    release!();
    await page.unrouteAll({ behavior: "wait" });
    await expect(everyone).toBeEnabled();
    await expect(
      page
        .getByText("Allowed Broadcasters", { exact: true })
        .locator("..")
        .locator("textarea"),
    ).toHaveValue("did:web:loaded-later.example");
    await expect(
      page.getByRole("checkbox", {
        name: "Mark your content as available for generative AI training",
      }),
    ).toBeChecked();
    await expect(duration).toHaveValue("7200");
    await saveMetadata(page);
    await page.reload();
    await expect(duration).toHaveValue("7200");
    await expect(
      page
        .getByText("Allowed Broadcasters", { exact: true })
        .locator("..")
        .locator("textarea"),
    ).toHaveValue("did:web:loaded-later.example");
    await expect(
      page.getByRole("checkbox", {
        name: "Mark your content as available for generative AI training",
      }),
    ).toBeChecked();
  } finally {
    release?.();
    await page.unrouteAll({ behavior: "wait" });
    await account.restore();
  }
});

test("edits made during a save do not replay previously saved fields", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const account = await metadataAccount();
  let release: (() => void) | undefined;
  try {
    await account.put({
      $type: collection,
      distributionPolicy: {
        allowedBroadcasters: ["did:web:original.example"],
        deleteAfter: 1800,
        allowGenAiTraining: false,
      },
    });
    await loginThroughPds(page);
    await page.goto(`${url}/dashboard/stream`);
    const broadcasters = page
      .getByText("Allowed Broadcasters", { exact: true })
      .locator("..")
      .locator("textarea");
    const duration = page
      .getByText("Delete After", { exact: true })
      .locator("..")
      .locator("input");
    await expect(duration).toHaveValue("1800");
    await duration.fill("7200");
    let intercepted!: () => void;
    const held = new Promise<void>((resolve) => (intercepted = resolve));
    const resume = new Promise<void>((resolve) => (release = resolve));
    await page.route("**/xrpc/com.atproto.repo.putRecord", async (route) => {
      if (route.request().postDataJSON()?.collection === collection) {
        intercepted();
        await resume;
      }
      await route.continue();
    });
    const firstSave = page.waitForResponse(
      (response) =>
        response.url().endsWith("/xrpc/com.atproto.repo.putRecord") &&
        response.request().postDataJSON()?.collection === collection,
    );
    await page
      .getByRole("button", { name: /save metadata|update metadata/i })
      .click();
    await held;
    await broadcasters.fill("did:web:edited-during-save.example");
    release!();
    expect((await firstSave).ok()).toBe(true);
    await page.unrouteAll({ behavior: "wait" });
    await expect(broadcasters).toHaveValue(
      "did:web:edited-during-save.example",
    );

    const other = await page.context().newPage();
    await other.goto(`${url}/dashboard/stream`);
    const otherDuration = other
      .getByText("Delete After", { exact: true })
      .locator("..")
      .locator("input");
    await expect(otherDuration).toHaveValue("7200");
    await otherDuration.fill("10800");
    await saveMetadata(other);
    await saveMetadata(page);
    await page.reload();
    await expect(broadcasters).toHaveValue(
      "did:web:edited-during-save.example",
    );
    await expect(duration).toHaveValue("10800");
  } finally {
    release?.();
    await page.unrouteAll({ behavior: "wait" });
    await account.restore();
  }
});

test("a real concurrent-save conflict keeps distribution edits for retry", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const account = await metadataAccount();
  let release: (() => void) | undefined;
  try {
    await account.put({
      $type: collection,
      distributionPolicy: {
        allowedBroadcasters: ["did:web:original.example"],
        deleteAfter: 1800,
        allowGenAiTraining: false,
      },
    });
    await loginThroughPds(page);
    await page.goto(`${url}/dashboard/stream`);
    const broadcasters = page
      .getByText("Allowed Broadcasters", { exact: true })
      .locator("..")
      .locator("textarea");
    const duration = page
      .getByText("Delete After", { exact: true })
      .locator("..")
      .locator("input");
    await expect(duration).toHaveValue("1800");
    await broadcasters.fill("did:web:draft.example");
    await duration.fill("7200");
    let intercepted!: () => void;
    const held = new Promise<void>((resolve) => (intercepted = resolve));
    const resume = new Promise<void>((resolve) => (release = resolve));
    await page.route("**/xrpc/com.atproto.repo.putRecord", async (route) => {
      if (route.request().postDataJSON()?.collection === collection) {
        intercepted();
        await resume;
      }
      await route.continue();
    });
    const failedSave = page.waitForResponse(
      (response) =>
        response.url().endsWith("/xrpc/com.atproto.repo.putRecord") &&
        response.request().postDataJSON()?.collection === collection,
    );
    await page
      .getByRole("button", { name: /save metadata|update metadata/i })
      .click();
    await held;
    const other = await page.context().newPage();
    await other.goto(`${url}/dashboard/stream`);
    const otherDuration = other
      .getByText("Delete After", { exact: true })
      .locator("..")
      .locator("input");
    await expect(otherDuration).toHaveValue("1800");
    await other
      .getByRole("checkbox", {
        name: "Mark your content as available for generative AI training",
      })
      .check();
    await saveMetadata(other);
    release!();
    expect((await failedSave).ok()).toBe(false);
    await expect(page.getByText("Settings changed elsewhere")).toBeVisible();
    await expect(broadcasters).toHaveValue("did:web:draft.example");
    await expect(duration).toHaveValue("7200");
    await page.unrouteAll({ behavior: "wait" });
    await saveMetadata(page);
    await page.reload();
    await expect(broadcasters).toHaveValue("did:web:draft.example");
    await expect(duration).toHaveValue("7200");
    await expect(
      page.getByRole("checkbox", {
        name: "Mark your content as available for generative AI training",
      }),
    ).toBeChecked();
  } finally {
    release?.();
    await page.unrouteAll({ behavior: "wait" });
    await account.restore();
  }
});
