import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("stream settings show all metadata sections and save edits together", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard/stream`);
  await expect(
    page.getByText("Content Warnings", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Content Rights", { exact: true })).toBeVisible();
  await expect(page.getByText("Distribution", { exact: true })).toBeVisible();
  await expect(page.getByText("Rights", { exact: true })).toHaveCount(0);

  const death = page.getByRole("checkbox", { name: "Death" });
  const drugUse = page.getByRole("checkbox", { name: "Drug Use" });
  const desktopDeath = (await death.boundingBox())!;
  const desktopDrugUse = (await drugUse.boundingBox())!;
  expect(desktopDrugUse.x).toBeGreaterThan(desktopDeath.x);
  expect(Math.abs(desktopDrugUse.y - desktopDeath.y)).toBeLessThan(2);

  const warning = page.getByRole("checkbox", { name: "Language" });
  const warningInitiallyChecked = await warning.isChecked();
  if (!warningInitiallyChecked) await warning.click();
  const notice = page.getByPlaceholder("Enter your copyright notice...");
  const noticeLabel = page.getByText("Copyright Notice", { exact: true });
  const labelRight = await noticeLabel.evaluate((element) => {
    const range = document.createRange();
    range.selectNodeContents(element);
    return range.getBoundingClientRect().right;
  });
  expect(labelRight).toBeLessThan((await notice.boundingBox())!.x);
  const originalNotice = await notice.inputValue();
  await notice.fill("Creator settings draft");
  const distribute = page.getByRole("checkbox", {
    name: "Allow everyone to distribute your content",
  });
  const distributeInitiallyChecked = await distribute.isChecked();
  if (!distributeInitiallyChecked) await distribute.click();
  const aiTraining = page.getByRole("checkbox", {
    name: "Mark your content as available for generative AI training",
  });
  await aiTraining.hover();
  await expect(
    page.getByText(
      "Your video's metadata signals whether you want your content used to train generative AI.",
    ),
  ).toBeVisible();
  const aiTrainingInitiallyChecked = await aiTraining.isChecked();
  if (!aiTrainingInitiallyChecked) await aiTraining.click();
  const saved = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      /\/xrpc\/com\.atproto\.repo\.(createRecord|putRecord)$/.test(
        response.url(),
      ) &&
      response.request().postDataJSON()?.collection ===
        "place.stream.metadata.configuration",
  );
  await page
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  expect((await saved).ok()).toBe(true);
  await page.reload();
  await expect(warning).toBeChecked();
  await expect(notice).toHaveValue("Creator settings draft");
  await expect(distribute).toBeChecked();
  await expect(aiTraining).toBeChecked();
  await distribute.click();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect
    .poll(async () => {
      const first = await death.boundingBox();
      const second = await drugUse.boundingBox();
      return first && second ? Math.abs(first.x - second.x) : Infinity;
    })
    .toBeLessThan(2);
  expect((await drugUse.boundingBox())!.y).toBeGreaterThan(
    (await death.boundingBox())!.y,
  );
  const phoneLabel = (await noticeLabel.boundingBox())!;
  expect((await notice.boundingBox())!.y).toBeGreaterThanOrEqual(
    phoneLabel.y + phoneLabel.height,
  );
  expect((await notice.boundingBox())!.height).toBeGreaterThanOrEqual(96);
  expect(
    (await page.getByPlaceholder("Enter your credit line...").boundingBox())!
      .height,
  ).toBeGreaterThanOrEqual(96);
  expect(
    (await page
      .getByText("Allowed Broadcasters", { exact: true })
      .boundingBox())!.height,
  ).toBeLessThan(30);
  if (!warningInitiallyChecked) await warning.click();
  if (distributeInitiallyChecked) await distribute.click();
  if (!aiTrainingInitiallyChecked) await aiTraining.click();
  await notice.fill(originalNotice);
  const restored = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url().endsWith("/xrpc/com.atproto.repo.putRecord") &&
      response.request().postDataJSON()?.collection ===
        "place.stream.metadata.configuration",
  );
  await page
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  expect((await restored).ok()).toBe(true);
});
