// Only the high-port e2e harness opts in. Node 22's built-in fetch does not
// otherwise honor HTTPS_PROXY; the default 443 path keeps its usual dispatcher.
if (process.env.DEV_ENV_HTTPS_PROXY === "true") {
  const { EnvHttpProxyAgent, setGlobalDispatcher } = await import("undici");
  setGlobalDispatcher(
    new EnvHttpProxyAgent({
      httpsProxy: process.env.HTTPS_PROXY,
      noProxy: process.env.NO_PROXY,
    }),
  );
}
