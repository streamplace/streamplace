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
  "active_manifest": "urn:c2pa:b23b55a7-bd34-4138-99d8-ce565fab3934",
  "manifests": {
    "urn:c2pa:b23b55a7-bd34-4138-99d8-ce565fab3934": {
      "claim_generator_info": [
        {
          "name": "c2pa-rs",
          "version": "0.58.0",
          "org.contentauth.c2pa_rs": "0.58.0"
        }
      ],
      "title": "Livestream Segment at 2025-10-21T19:24:24.156Z",
      "instance_id": "xmp:iid:17f3177c-7cfe-4de2-9a23-019dcdb00559",
      "ingredients": [],
      "assertions": [
        {
          "label": "c2pa.actions.v2",
          "data": {
            "actions": [
              {
                "action": "c2pa.created"
              },
              {
                "action": "c2pa.published"
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
            "hash": "HrLwGm+HdaZh9TkBiWhJH1Mo7QcvLgmhMThcG8f3qZc=",
            "name": "jumbf manifest"
          }
        },
        {
          "label": "place.stream.metadata",
          "data": {
            "@context": {
              "photoshop": "http://ns.adobe.com/photoshop/1.0/",
              "dc": "http://purl.org/dc/elements/1.1/",
              "xmpRights": "http://ns.adobe.com/xap/1.0/rights/",
              "Iptc4xmpExt": "http://iptc.org/std/Iptc4xmpExt/2008-02-29/"
            },
            "dc:creator": "did:plc:y3lae7hmqiwyq7w2v3bcb2c2",
            "dc:title": ["🦎🦎"],
            "dc:date": ["2025-10-21T19:24:24.156Z"],
            "distributionPolicy": {
              "deleteAfter": "2025-10-21T19:29:24.000Z"
            },
            "Iptc4xmpExt:LinkedEncRightsExpr": "http://creativecommons.org/publicdomain/zero/1.0/"
          },
          "kind": "Json"
        },
        {
          "label": "place.stream.metadata.configuration",
          "data": {
            "$type": "place.stream.metadata.configuration",
            "contentRights": {
              "license": "place.stream.metadata.contentRights#cc0_1__0"
            },
            "distributionPolicy": {
              "deleteAfter": 300
            }
          }
        },
        {
          "label": "place.stream.livestream",
          "data": {
            "url": "https://picnic-labs-nicholas-not.trycloudflare.com",
            "post": {
              "cid": "bafyreicucf722xnyf74psia5ghd5usdnona4e7bkcgbqhdma2a6dokqh5m",
              "uri": "at://did:plc:y3lae7hmqiwyq7w2v3bcb2c2/app.bsky.feed.post/3lxyfybn55m2o"
            },
            "$type": "place.stream.livestream",
            "thumb": {
              "ref": {
                "$link": "bafkreiauoc74hcintbaua7tvp233qbfl4iymiyocc5aclhyohkz3bdinty"
              },
              "size": 9231,
              "$type": "blob",
              "mimeType": "image/jpeg"
            },
            "title": "🦎🦎",
            "createdAt": "2025-10-06T16:25:06.950Z"
          }
        }
      ],
      "signature_info": {
        "issuer": "Streamplace",
        "common_name": "did:key:zQ3shfmFgwDstMiGaAkS4HhMJ7p3pTVhyLTHz9ABbhd4v4KJn",
        "cert_serial_number": "54472225560857906834076190516168844896"
      },
      "label": "urn:c2pa:b23b55a7-bd34-4138-99d8-ce565fab3934"
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
