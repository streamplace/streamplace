export default function getEnv() {
  let updateBaseUrl = "https://stream.place";
  if (process.env["AQD_UPDATE_BASE_URL"]) {
    updateBaseUrl = process.env["AQD_UPDATE_BASE_URL"];
  }
  const isDev = process.env["WEBPACK_SERVE"] === "true";
  if (!isDev) {
    return {
      isDev: false,
      skipNode: false,
      nodeFrontend: true,
      updateBaseUrl,
    };
  }
  return {
    isDev: process.env["WEBPACK_SERVE"] === "true",
    skipNode: process.env["AQD_SKIP_NODE"] !== "false",
    nodeFrontend: process.env["AQD_NODE_FRONTEND"] === "true",
    noUpdate: process.env["AQD_NO_UPDATE"] === "true",
    updateBaseUrl,
  };
}
