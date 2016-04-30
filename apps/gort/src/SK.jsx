import SKClient from "sk-client";

if (!window.SK_PARAMS || !window.SK_PARAMS.API_SERVER_URL) {
  throw new Error("Missing required environment variable: API_SERVER_URL");
}

const SK = new SKClient({
  server: window.SK_PARAMS.API_SERVER_URL,
  log: true
});

export default SK;

// Ehhhhh. Makes debugging easier. Who cares.
window.SK = SK;
