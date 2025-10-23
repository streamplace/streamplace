---
title: "How Signing Works"
sidebar:
  order: 20
---

The signing process integrates the user's identity and preferences with the C2PA
standard to produce a verifiable stream. At a high level, this is how it works:

1. **Key Generation**: The user clicks "Generate Stream Key" on the Streamplace
   frontend to create a `secp256k1` keypair.
2. **Key Distribution**: The user is given a stream key that includes the
   private key combined with their DID, encoded in a multibase format. The
   corresponding public key is stored in the user's PDS as a `place.stream.key`
   AT Protocol record for public verification.
3. **Node Synchronization (Key)**: When the `place.stream.key` record is
   created, the AT Protocol firehose picks it up. Streamplace nodes then sync
   this record to a local SQLite database.
4. **Metadata Configuration**: In a similar process, the user creates a
   `place.stream.metadata.configuration` record via the frontend. This record
   contains the user's preferences for **content warnings**, **content rights**,
   and **distribution policy**. This record is also synced by nodes to their
   local database.
5. **Stream Authentication**: When a user starts a stream, they include their
   stream key as a param in the WHIP or RTMPS request to the node. The node
   decodes the key, extracts the private key and DID, and verifies that the
   public key exists and is valid.
6. **Signer Creation**: Once authenticated, the node creates a signer instance
   using the user's private key.
7. **Segmentation**: The incoming live stream is segmented into one-second MP4
   chunks.
8. **Manifest Creation and Signing**: For each segment, the node creates a C2PA
   manifest using the user's metadata configuration. It then uses the streamer's
   private key to sign the manifest, and embeds the signed manifest directly
   into the MP4 segment.
9. **Signed Segments**: The output is a continuous stream of MP4 segments, each
   cryptographically signed and containing its own C2PA manifest.
