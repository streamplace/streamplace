---
title: "Introduction"
sidebar:
  order: 10
---

Every stream on Streamplace is cryptographically bound with metadata that
captures details about how the stream should be viewed, used, distributed, and
monetized. This metadata also identifies the provenance of the stream, including
who the creator is and any transformations it has undergone. Any Streamplace
node operator can inspect the stream and verify that this metadata is intact and
trustworthy.

This means that even when Streamplace video is downloaded or redistributed, it
can still be provably linked back to the original streamer, as long as metadata
wasn't stripped. This is a powerful property that allows for sourcing,
fact-checking, remixing, and more, all with attribution built-in.

The technical standard Streamplace has adopted for this is the
[Coalition for Content Provenance and Authenticity](https://c2pa.org/) (C2PA).
The benefit of adhering to a standard is that it's a well-vetted specification
developed by many organizations in the digital media space, which means
Streamplace doesn't need to reinvent the wheel. It also provides
interoperability as more companies, devices, and software ecosystems adopt the
same standard.

However, the current Streamplace implementation isn't fully compliant with the
C2PA standard. Streamplace uses the ES256K algorithm (ECDSA with SHA-256 over
`secp256k1`) for signing, which aligns with the cryptographic standards of the
AT Protocol and other decentralized systems but is not yet officially supported
by C2PA. We hope the standard will recognize this algorithm in the future to
simplify integration with decentralized protocols.
