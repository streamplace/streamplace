import assert from "node:assert/strict";
import { writeFile } from "node:fs/promises";
import { join } from "node:path";
import { setTimeout as delay } from "node:timers/promises";

const {
  ACCOUNT_HANDLE: handle,
  ACCOUNT_PASSWORD: password,
  SERVER_HTTPS_URL: station,
  PDS_HTTPS_URL: pds,
  E2E_ARTIFACTS: artifacts,
  E2E_IOS_DEVICE: device,
  SAFARIDRIVER_PORT: port,
} = process.env;
assert(handle && password && station && pds && artifacts && device && port);

const endpoint = `http://127.0.0.1:${port}`;
const elementKey = "element-6066-11e4-a52e-4f735466cecf";
class WebDriverError extends Error {
  readonly code: string;

  constructor(code: string, message: string) {
    super(message);
    this.code = code;
  }
}

async function command<T>(path: string, body?: object, method?: string) {
  const response = await fetch(`${endpoint}${path}`, {
    method: method ?? (body ? "POST" : "GET"),
    headers: body ? { "content-type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
    signal: AbortSignal.timeout(path === "/session" ? 90_000 : 35_000),
  });
  const { value } = await response.json();
  if (!response.ok) {
    throw new WebDriverError(value.error, `${path}: ${value.message}`);
  }
  return value as T;
}

async function until<T>(
  description: string,
  check: () => Promise<T | false>,
  timeout = 30_000,
): Promise<T> {
  const deadline = performance.now() + timeout;
  do {
    const result = await check();
    if (result !== false) return result;
    await delay(200);
  } while (performance.now() < deadline);
  throw new Error(`Timed out waiting for ${description}`);
}

await until(
  "SafariDriver",
  async () => {
    try {
      return (await command<{ ready: boolean }>("/status")).ready;
    } catch (error) {
      // The server has not bound its port yet.
      if (error instanceof TypeError && error.message === "fetch failed") {
        return false;
      }
      throw error;
    }
  },
  10_000,
);
const { sessionId, capabilities } = await command<{
  sessionId: string;
  capabilities: Record<string, unknown>;
}>("/session", {
  capabilities: {
    alwaysMatch: {
      browserName: "Safari",
      platformName: "iOS",
      "safari:useSimulator": true,
      "safari:deviceUDID": device,
    },
  },
});
const session = `/session/${sessionId}`;
const send = <T,>(path: string, body?: object) =>
  command<T>(`${session}${path}`, body);
const go = (url: string) => send("/url", { url });
const url = () => send<string>("/url");

async function find(selector: string) {
  try {
    const element = await send<Record<typeof elementKey, string>>("/element", {
      using: selector.startsWith("//") ? "xpath" : "css selector",
      value: selector,
    });
    const path = `/element/${element[elementKey]}`;
    return (await send<boolean>(`${path}/displayed`)) ? path : false;
  } catch (error) {
    if (
      error instanceof WebDriverError &&
      ["no such element", "stale element reference"].includes(error.code)
    ) {
      return false;
    }
    throw error;
  }
}

const visible = (selector: string) => until(selector, () => find(selector));
const click = (element: string) => send(`${element}/click`, {});
const type = async (selector: string, text: string) => {
  await send(`${await visible(selector)}/value`, { text });
};
const id = (name: string) => `[data-testid="${name}"]`;
const text = (value: string) => `//*[text()=${JSON.stringify(value)}]`;
const input = (name: string) => `input${id(name)}, ${id(name)} input`;
const button = (...labels: string[]) => {
  const label =
    "translate(normalize-space(.), 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz')";
  return `//button[${labels.map((value) => `${label}='${value}'`).join(" or ")}]`;
};

try {
  assert.equal(capabilities["safari:deviceUDID"], device);
  console.log(`Safari ${capabilities.browserVersion} on iPhone ${device}`);
  await send("/timeouts", { pageLoad: 30_000 });

  console.log("Pointing the app at the harness");
  await go(`${station}/settings/advanced`);
  await click(await visible(id("settings-use-custom-node")));
  await type(input("settings-custom-node-url"), station);
  await click(await visible(id("settings-save-node")));
  await until("node URL to persist", () =>
    send<boolean>("/execute/sync", {
      script:
        "return Object.values(localStorage).some(v => v === arguments[0] || v === JSON.stringify(arguments[0]));",
      args: [station],
    }),
  );

  console.log("Signing in through the fixture PDS");
  await go(`${station}/login`);
  await type(input("login-handle"), handle);
  await click(await visible(id("login-submit")));
  await until(
    "PDS sign-in page",
    async () => new URL(await url()).origin === pds,
  );
  await type('input[type="password"]', password);
  await click(await visible(button("sign in", "next")));
  const consent = await until("PDS consent or OAuth callback", async () => {
    if (new URL(await url()).origin === station) return null;
    return find(button("authorize"));
  });
  if (consent) await click(consent);
  await visible(text(`@${handle}`));

  console.log("Sending chat and checking reloaded history");
  await go(station);
  await click(await visible(id("home-stream-card")));
  const chatInput = input("chat-input");
  await visible(text("Now streaming - e2e test stream"));
  const message = `hello from iPhone Safari ${Date.now()}`;
  const chatMessage = `//*[@data-testid="chat-list"]${text(message)}`;
  await type(chatInput, message);
  const sendButton = await until("chat composer to be ready", async () => {
    const element = await find(id("chat-send"));
    return (
      element &&
      (await send<boolean>(`${element}/enabled`)) &&
      (await send<string | null>(`${element}/attribute/aria-disabled`)) !==
        "true" &&
      element
    );
  });
  await click(sendButton);
  await visible(chatMessage);
  assert.equal(await send(`${await visible(chatInput)}/property/value`), "");

  // The first appearance may be optimistic. Reload to read the node's history.
  await send("/refresh", {});
  await visible(chatMessage);
  console.log("Passed: chat persisted after reload");
} catch (error) {
  try {
    const screenshot = await send<string>("/screenshot");
    await writeFile(
      join(artifacts, "safari.png"),
      Buffer.from(screenshot, "base64"),
    );
  } catch (screenshotError) {
    console.error("Could not capture Safari screenshot:", screenshotError);
  }
  throw error;
} finally {
  await command(session, undefined, "DELETE");
}
