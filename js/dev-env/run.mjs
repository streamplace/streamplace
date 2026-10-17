import { TestNetwork } from "./dist/index.js";

// With DEV_ENV_PDS_HOSTNAME the PDS presents itself as https://<hostname>
// (its OAuth issuer, did:web and account handles under .<hostname>) while
// still listening on plain HTTP; whoever sets it terminates TLS in front of
// it. `streamplace e2e --https-pds-hostname` does, see pkg/cmd/e2e_https.go.
//
// In that mode there is also a lexicon authority account, the local stand-in
// for the did:plc account that _lexicon.stream.place points at in production:
// the PDS resolves every lexicon (OAuth permission sets) from it. Its
// credentials are printed for the caller to publish into.
const hostname = process.env.DEV_ENV_PDS_HOSTNAME;
const lexiconPassword = "lexicons";

(async () => {
  const network = await TestNetwork.create(
    hostname
      ? { pds: { hostname, serviceHandleDomains: [`.${hostname}`] } }
      : {},
  );
  const out = { "pds-url": network.pds.url, "plc-url": network.plc.url };
  if (hostname) {
    const res = await fetch(
      `${network.pds.url}/xrpc/com.atproto.server.createAccount`,
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          handle: `lexicons.${hostname}`,
          email: `lexicons@${hostname}`,
          password: lexiconPassword,
        }),
      },
    );
    if (!res.ok) {
      throw new Error(`create lexicon account: ${await res.text()}`);
    }
    const { did } = await res.json();
    // PDS_LEXICON_AUTHORITY_DID, set after the fact: the account can only be
    // created once the PDS is up, and the PDS reads this per lookup.
    network.pds.ctx.cfg.lexicon.didAuthority = did;
    out["lexicon-did"] = did;
    out["lexicon-password"] = lexiconPassword;
  }
  console.log(JSON.stringify(out));
})();
