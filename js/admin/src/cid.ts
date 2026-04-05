/**
 * BDASL CID computation — mirrors the Rust bdasl_cid function.
 *
 * CIDv1 with raw codec (0x55), BLAKE3 hash (0x1e), base32lower encoding.
 */

const BASE32_ALPHABET = "abcdefghijklmnopqrstuvwxyz234567";

function base32LowerEncode(data: Uint8Array): string {
  let result = "";
  let buffer = 0;
  let bits = 0;
  for (const byte of data) {
    buffer = (buffer << 8) | byte;
    bits += 8;
    while (bits >= 5) {
      bits -= 5;
      result += BASE32_ALPHABET[(buffer >> bits) & 0x1f];
    }
  }
  if (bits > 0) {
    result += BASE32_ALPHABET[(buffer << (5 - bits)) & 0x1f];
  }
  return result;
}

/**
 * Compute a BDASL CID from a BLAKE3 digest (32 bytes).
 */
export function bdaslCidFromDigest(digest: Uint8Array): string {
  // CID binary: version(1) + codec(0x55=raw) + hash_fn(0x1e=blake3) + hash_len(0x20) + digest
  const cidBytes = new Uint8Array(4 + 32);
  cidBytes[0] = 0x01;
  cidBytes[1] = 0x55;
  cidBytes[2] = 0x1e;
  cidBytes[3] = 0x20;
  cidBytes.set(digest, 4);
  return "b" + base32LowerEncode(cidBytes);
}
