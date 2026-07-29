// Package reposync implements a verifiable, resumable, prefix-bounded walk of a
// remote atproto repository's Merkle Search Tree (MST).
//
// The motivating problem: streamplace only cares about the `place.stream.*`
// records in an account's repo, but the only "repair" primitive atproto gives us
// out of the box is `com.atproto.sync.getRepo`, which downloads the entire repo
// as a CAR. Because MST keys are `collection/rkey` strings sorted bytewise, every
// record we care about lives in one contiguous key range, so we can instead walk
// only the subtrees that overlap that range: O(records-we-want + log n) blocks
// instead of O(repo).
//
// # Verification
//
// Every block is dag-cbor addressed by CID. A [BlockFetcher] must check that the
// bytes it returns hash to the CID that was requested, so the whole walk is
// chained to the CID in the signed commit ([FetchVerifiedHead]). A remote PDS
// therefore cannot forge record contents, nor can it silently omit a record from
// the walked range: an omission shows up as a missing block, which is an error.
//
// # Semantics
//
//   - Completeness. When [Walker.WalkPrefix], [Walker.WalkRanges] or
//     [Walker.Resume] return nil, the set of records passed to the visitor is
//     exactly the set of in-range keys in the tree rooted at the given root.
//     A block the server does not return is an error ([ErrMissingBlock]), never
//     a skip. This is deliberately unlike indigo's partial-tolerant
//     mst.LoadTreeFromStore.
//
//   - At-least-once emission. A walk checkpoints its [Frontier] only after the
//     records for that step have been handed to the visitor. Interrupting a walk
//     and resuming from the last checkpoint therefore re-emits the records of the
//     step that was in flight. Visitors must be idempotent, keyed by
//     (path, record CID).
//
//   - Key order. Records are emitted in ascending bytewise key order across the
//     whole walk, not merely within a step.
//
//   - Deletions are not observable from a single walk; a caller detects them by
//     diffing two walks (see [Walker.CollectPrefix] and [DiffCollections]).
//
// # Transient failures
//
// Walking a large repo is hundreds of sequential getBlocks calls, so a single
// rate limit or restarting host must not end it: [XRPCBlockFetcher] and
// [FetchVerifiedHead] retry 429s, 5xx and dropped connections with a jittered
// exponential backoff ([RetryPolicy]). Everything else -- 4xx, verification
// failures, a cancelled context -- fails immediately.
//
// Guessing at the backoff is the last resort, not the first: a host that
// answers 429 or 503 usually says when to come back, and [BackoffHints] is how
// that gets read. Install its [BackoffHints.Transport] on the http.Client behind
// the xrpc.Client and point [RetryPolicy.Hints] at the same registry; waits then
// honor Retry-After and ratelimit-reset instead of a ladder. Without it the
// headers are simply lost -- indigo's xrpc client keeps a status code and
// discards the response headers.
//
// One 4xx in particular is worth knowing about: a walk pins a root and then
// reads it over many round trips, while the host garbage-collects blocks that
// only superseded commits referenced. A repo that commits mid-walk can leave
// blocks unfetchable ([ErrMissingBlock], or a host-specific BlockNotFound /
// "Could not find cids"). Retrying cannot help; the caller has to re-read the
// head and walk the new tree, reusing its [CachedFetcher] so the second pass
// only pays for what changed.
//
// # Resuming
//
// A [Frontier] is the complete state of an in-progress walk and is JSON
// serializable, so it can be persisted and picked up in another process. Pair a
// resumed walk with a [CachedFetcher] over a warm [BlockCache] and the already
// walked part of the tree costs no network traffic at all.
package reposync
