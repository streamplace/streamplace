// QR login, the browser's half, from the host: QR_LOGIN_STEP=create starts an
// attempt as a browser would (output.qrLoginId); QR_LOGIN_STEP=check asserts
// the app's answer reached the node. A real browser's attempt carries its
// OAuth pushed authorization request, which only approval looks at; denying
// doesn't, so a placeholder stands in here (approval is covered on the web,
// js/e2e-web/flows/login-options.spec.ts, by the same screen).
function call(method, url, body) {
  const res = http.request(url, {
    method,
    headers: {
      "Content-Type": "application/json",
      "User-Agent":
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(`${url}: HTTP ${res.status}: ${res.body}`);
  }
  return json(res.body);
}

if (QR_LOGIN_STEP === "create") {
  const { id } = call(
    "POST",
    `${SERVER_HTTP_URL}/xrpc/place.stream.login.createQRLogin`,
    { requestUri: "urn:ietf:params:oauth:request_uri:e2e-placeholder" },
  );
  output.qrLoginId = id;
} else {
  const login = call(
    "GET",
    `${SERVER_HTTP_URL}/xrpc/place.stream.login.getQRLogin?id=${encodeURIComponent(output.qrLoginId)}`,
  );
  if (login.status !== "denied" || login.authorization) {
    throw new Error(`expected a denied attempt, got ${JSON.stringify(login)}`);
  }
}
