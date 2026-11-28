// Registers the harness's real pushed authorization request, then verifies
// that the signed-in app denied it. Web approval is covered by qr-login.spec.ts.
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
    { requestUri: E2E_QR_REQUEST_URI },
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
