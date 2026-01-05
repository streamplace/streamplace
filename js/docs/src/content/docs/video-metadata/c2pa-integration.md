---
title: "C2PA Integration"
sidebar:
  order: 40
---

The actual metadata insertion and signing done to each video segment is through
[C2PA](https://c2pa.org/) tooling. A
[fork](https://github.com/streamplace/c2pa-rs) of their Rust SDK that adds
support for ES256K is used, and called in Go through FFI. The code for this can
be seen in `rust/iroh-streamplace/src/c2pa.rs`.

The metadata stored in the video with C2PA must be in a valid C2PA "manifest"
format, documented in the specification. Here is an example Streamplace
manifest, extracted from the MP4 segment using
[c2patool](https://github.com/contentauth/c2pa-rs/tree/main/cli).

```json
{
  "active_manifest": "urn:c2pa:66130743-a4cd-4f73-a4c7-201079ef127a",
  "manifests": {
    "urn:c2pa:66130743-a4cd-4f73-a4c7-201079ef127a": {
      "claim_generator_info": [
        {
          "name": "c2pa-rs",
          "version": "0.58.0",
          "org.contentauth.c2pa_rs": "0.58.0"
        }
      ],
      "title": "Livestream Segment at 2025-11-13T20:41:30.499Z",
      "instance_id": "xmp:iid:a031a30d-e1eb-4548-a20f-6453e15c2ace",
      "ingredients": [],
      "assertions": [
        {
          "label": "c2pa.actions.v2",
          "data": {
            "actions": [
              {
                "action": "c2pa.created",
                "when": "2025-11-13T20:41:30.499Z"
              },
              {
                "action": "c2pa.published",
                "when": "2025-11-13T20:41:30.499Z"
              }
            ],
            "allActionsIncluded": false
          }
        },
        {
          "label": "c2pa.hash.bmff.v3",
          "data": {
            "exclusions": [
              {
                "xpath": "/uuid",
                "length": null,
                "data": [
                  {
                    "offset": 8,
                    "value": "2P7D1hsOSDySl1goh37EgQ=="
                  }
                ],
                "subset": null,
                "version": null,
                "flags": null,
                "exact": null
              },
              {
                "xpath": "/ftyp",
                "length": null,
                "data": null,
                "subset": null,
                "version": null,
                "flags": null,
                "exact": null
              },
              {
                "xpath": "/mfra",
                "length": null,
                "data": null,
                "subset": null,
                "version": null,
                "flags": null,
                "exact": null
              }
            ],
            "alg": "sha256",
            "hash": "ZHobn5CtL1NsTLAMMPT8D7koSrMr2Ijm5eJt73ED4HA=",
            "name": "jumbf manifest"
          }
        },
        {
          "label": "cawg.metadata",
          "data": {
            "@context": {
              "photoshop": "http://ns.adobe.com/photoshop/1.0/",
              "xmpRights": "http://ns.adobe.com/xap/1.0/rights/",
              "Iptc4xmpExt": "http://iptc.org/std/Iptc4xmpExt/2008-02-29/",
              "dc": "http://purl.org/dc/elements/1.1/"
            },
            "xmpRights:UsageTerms": "All rights reserved",
            "dc:creator": "did:plc:2j2ounbiyi3ftihronlw5qhj",
            "dc:date": "2025-11-13T20:41:30.499Z",
            "Iptc4xmpExt:ContentWarning": ["cwarn:flashingLights"],
            "dc:title": "Test"
          },
          "kind": "Json"
        },
        {
          "label": "place.stream.metadata.configuration",
          "data": {
            "$type": "place.stream.metadata.configuration",
            "contentRights": {
              "license": "place.stream.metadata.contentRights#all-rights-reserved"
            },
            "contentWarnings": {
              "warnings": ["cwarn:flashingLights"]
            },
            "distributionPolicy": {
              "deleteAfter": 300
            }
          }
        },
        {
          "label": "place.stream.livestream",
          "data": {
            "url": "https://headphones-glad-thunder-guide.trycloudflare.com",
            "post": {
              "cid": "bafyreigxivgp5rjgohv7y6z3r4wdjw6qbu56ozartttuvtydoth45app7e",
              "uri": "at://did:plc:2j2ounbiyi3ftihronlw5qhj/app.bsky.feed.post/3m2k26c2l4u2g"
            },
            "$type": "place.stream.livestream",
            "agent": "@streamplace/components/0.7.35 (web, Firefox)",
            "thumb": {
              "ref": {
                "$link": "bafkreihzmf7rywllxelclvnweuefl2bpqkkazznflnhdh24aexnqzh4xum"
              },
              "size": 60106,
              "$type": "blob",
              "mimeType": "image/jpeg"
            },
            "title": "Test",
            "createdAt": "2025-10-06T16:35:03.719Z",
            "canonicalUrl": "https://headphones-glad-thunder-guide.trycloudflare.com/makeworld.space"
          }
        }
      ],
      "signature_info": {
        "issuer": "Streamplace",
        "common_name": "did:key:zQ3shiYS17LRhT7x6mfd6HfsHHzz1aD9DpGJUP3aT5f2ghdAy",
        "cert_serial_number": "34409281865771434870888029768053492928"
      },
      "label": "urn:c2pa:66130743-a4cd-4f73-a4c7-201079ef127a"
    }
  }
}
```

The official version of c2patool can extract this manifest, but will not
consider it valid due to the use of ES256K. If you build c2patool from the
[fork](https://github.com/streamplace/c2pa-rs) used by Streamplace, it will
validate.

Note the variety of information stored in the manifest: user DID, signing key,
timestamp, content warnings, copyright, etc. More can be added in the future,
for example whether you consent to remixing.

You can see several assertions with the name `place.stream.*`. This is where
Streamplace-specific metadata is stored, and is related to the lexicon. It's the
easiest place to parse out this metadata.

In addition to the primary `place.stream` assertions, we make a best-effort
attempt to translate the Streamplace assertions into spec-compliant C2PA
metadata assertions, which are also included in the signed manifest. This allows
other C2PA-compliant software to parse out information about Streamplace
segments, such as content warnings. However, not everything Streamplace does
fits neatly into C2PA-compliant metadata, so the primary source of truth for
metadata on a Streamplace segment remains the `place.stream` assertions.

You can see existing metadata standards being used in the `cawg.metadata`
assertion, which is an assertion defined outside of the C2PA spec for extra
metadata, by the [Creator Assertions Working Group](https://cawg.io/).

## Code paths

The C2PA manifest is built from the existing livestream metadata in
`pkg/media/manifest_builder.go`. This `ManifestBuilder` does the work of mapping
metadata settings from the user stored in the database into C2PA manifest
information. This is used by the media signer (`pkg/media/media_signer.go`)
which calls the Go-Rust wrapper to actually perform the C2PA injection into the
MP4, located at `pkg/iroh/generated/iroh_streamplace/iroh_streamplace.go` and
`rust/iroh-streamplace/src/c2pa.rs`.

The stream key is what is used for C2PA signing.

## Transcoding

C2PA supports linking media in a hierarchy, where one piece of media can derive
from one or more parents. Although this is not supported by Streamplace yet, in
the future it will be possible to add this information into video metadata when
a segment gets transcoded. The output video segment will contain a
`c2pa.transcoded` action that links back to the original video segment.
