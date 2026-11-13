import { expect, test } from "./fixture";
import { loginThroughSystemBrowser } from "./login";

test("shows the connected upstream MoQ destination", async ({ desktop }) => {
  const { page, nodeURL } = desktop;
  await loginThroughSystemBrowser(desktop);
  await page.goto(`${nodeURL}/desktop`);
  const broadcaster = await page.evaluate(async (stationURL) => {
    const response = await fetch(
      `${stationURL}/xrpc/place.stream.broadcast.getBroadcaster`,
    );
    if (!response.ok) throw new Error(`getBroadcaster: ${response.status}`);
    return (await response.json()) as { moqURL: string };
  }, process.env.UPSTREAM_STATION_URL!);
  const destination = new URL(broadcaster.moqURL);
  destination.search = "";
  destination.hash = "";
  await expect(page.getByTestId("desktop-forwarding-url")).toHaveText(
    destination.toString(),
    { timeout: 60_000 },
  );
  await expect(page.getByTestId("desktop-forwarding-state")).toContainText(
    `Connected to ${destination}`,
    { timeout: 60_000 },
  );
});
