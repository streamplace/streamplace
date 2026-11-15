import { access, readFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "./fixture";
import { loginThroughSystemBrowser } from "./login";

declare global {
  var __streamplaceOpenLogPaths: string[] | undefined;
}

test("save node logs and open their folder", async ({ desktop }) => {
  const app = desktop.electronApp();
  const logDir = await app.evaluate(({ app }) => app.getPath("logs"));
  const logPath = join(logDir, "node.log");
  await access(logPath);
  await expect.poll(() => readFile(logPath, "utf8")).toContain("runtime.GOOS");

  await loginThroughSystemBrowser(desktop);
  await app.evaluate(({ shell }) => {
    const opened: string[] = [];
    globalThis.__streamplaceOpenLogPaths = opened;
    shell.openPath = async (path: string) => {
      opened.push(path);
      return "";
    };
  });
  await desktop.page.getByTestId("desktop-open-logs").click();
  await expect
    .poll(() => app.evaluate(() => globalThis.__streamplaceOpenLogPaths))
    .toEqual([logDir]);
});
