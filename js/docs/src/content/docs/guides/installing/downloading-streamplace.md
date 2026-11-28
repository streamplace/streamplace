---
title: Downloading Streamplace
description: How to download Streamplace
sidebar:
  order: 20
---

## macOS

```shell
brew install streamplace/streamplace/streamplace
```

## Linux

### Debian/Ubuntu

```shell
sudo mkdir -p /etc/apt/keyrings
curl https://release.stream.place/streamplace.key | sudo gpg --dearmor -o /etc/apt/keyrings/streamplace.key
echo 'deb [signed-by=/etc/apt/keyrings/streamplace.key] https://release.stream.place/debian/ all main' \
  | sudo tee /etc/apt/sources.list.d/streamplace.list
sudo apt update
sudo apt install streamplace
```

This will install the `streamplace` systemd service. To configure it, you will
want to edit the environment variables at `/etc/streamplace/streamplace.env`. An
example production env file might look something like this:

```ini
# Handle default HTTP and HTTPS traffic for the server
SP_HTTP_ADDR=:80
SP_HTTPS_ADDR=:443
SP_SECURE=true

# Set this variable to your did:plc or did:web to have admin access to the node
SP_ADMIN_DIDS=did:web:example.com,did:plc:rbvrr34edl5ddpuwcubjiost

# If you're running Streamplace behind an HTTPS proxy, you'll want
# SP_SECURE=false
# SP_BEHIND_HTTPS_PROXY=true

# Necessary to advertise a public Streamplace broadcaster
SP_BROADCASTER_HOST=example.com
# If you have a multi-node cluster, they'll each need different public DNS names:
SP_SERVER_HOST=prod-nyc0.example.com

# If you don't want to syndicate everyone, add your list of allowed DIDs here:
SP_ALLOWED_STREAMS=did:web:example.com,did:plc:rbvrr34edl5ddpuwcubjiost

# Useful if your TLS cert and key aren't in the default
SP_TLS_CERT=/tls/tls.crt
SP_TLS_KEY=/tls/tls.key
```

Set `SP_ATPROTO_DID=did:plc:...` (or `--atproto-did`) to override the DID returned
by `/.well-known/atproto-did` on all node hosts, including the broadcaster host
in a multi-node cluster. Without an override, discovery returns the host-specific
`did:web` identity. This does not change the node's repository DIDs or DID documents.

### Object-backed station (Linux prototype)

Use one private S3-compatible bucket per station. Keep the normal SQLite/file
configuration for a single node: it does not require S3, SlateDB, or an
external PDS. For the opt-in Linux executable, run `make station-release`
in the builder and deploy `build-station-release-linux-<arch>/streamplace`.
Use `make dev-station` for a development checkout, not for deployment.

| Authority                                | What it holds                                                                       | If unavailable                                                 |
| ---------------------------------------- | ----------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| Creator's public repo and personal space | Published video records and granted private creator metadata                        | Private edits fail; no fallback to stale station copies        |
| Station account on an external PDS       | Public media origins and enabled VOD view-count reports                             | New announcements wait; existing eligible VOD can still play   |
| Station bucket                           | Media, recording manifests, creator shard state and encrypted operational SQL state | State-changing requests cannot safely proceed                  |
| Node disk and RAM                        | Rebuildable AppView and hot media                                                   | Warm the index on replacement; a live HLS window is node-local |

On every node set the same `SP_BROADCASTER_HOST`, `SP_STATION_DID`,
`SP_DB_URL=station://my-station`, `SP_S3_ENDPOINT`, `SP_S3_REGION`,
`SP_S3_BUCKET`, and S3 credentials. Give **each node** a different
`SP_SERVER_HOST` with a direct HTTPS endpoint. Set `SP_SECURE=true` for
native TLS, or `SP_BEHIND_HTTPS_PROXY=true` behind your TLS terminator.
Keep the internal/admin listener private. `SP_STATION_STATE_KEY_FILE` points
to the same 32 **raw bytes** on each node. Generate it once (`umask 077; openssl rand -out station.key 32`),
distribute through your secret manager, and back it up separately. Losing
it loses access to the encrypted state; replacing it is not key rotation.

Set `SP_STATION_PEERS` to comma-separated `DID=HTTPS-origin` entries for
the other nodes, for example `did:web:node2.example.com=https://node2.example.com`.
Use the same public OAuth issuer on every node. Peer addresses are a trust
boundary: point them directly at nodes you operate, not at the load balancer.

Draft and multistream management requests can enter through any node. The
creator DID hashes to one of 256 fixed slots; the first authenticated request
to an unowned slot activates its writer. Other nodes forward to that writer
over HTTPS. Adding an edge does not move existing slots or require a data
copy. After an owner crashes, a replacement can claim an expired slot
(30-second default lease) and reopen its SlateDB from the bucket. This is
demand-driven takeover, not an automatic load balancer for hot creators.

Configure the external account's DID document with **two distinct services**:
`#atproto_pds` for its repository and `#streamplace` for the station's public
HTTPS origin. Do not point the PDS service at a playback node. Enable
`SP_STATION_ORIGIN_PUBLISHER=true` on the designated publisher and authorize
the station account there. Do not move the account's signing or rotation key
onto the nodes. The station advertises stored VOD; the ingest node remains
the origin of a live broadcast.

If aggregating VOD views, configure `--view-count-aggregate-interval` on
the publisher and grant create/update on `place.stream.media.viewCount`.
Cut over at a closed reporting window after draining old jobs; historical
node-authored reports keep their original identity. Live viewer status is
ephemeral, not a station aggregate.

**Bucket requirements.** Use strongly consistent GET and LIST, conditional
PUT and DELETE, and multipart media uploads. Leave `--vod-cdn-url` unset:
public URLs that bypass a station node also bypass its withdrawal checks.
Keep credentials scoped to the station's bucket. Do not expose the bucket or
mount it as a public CDN origin: uploads and draft bytes are private even
when their content IDs are known. Do not configure object expiry on state,
manifests, withdrawal markers or media still referenced by records. A
namespace isolates the operational DB and shards, not the media keyspace.

Nodes retry concurrent state writes only when the bucket confirms that the
write did not commit. OAuth changes still reject replayed proofs and stale
credentials; an unacknowledged commit is reported rather than retried.

**Existing VOD and withdrawals.** Warm the AppView, then `POST /reindex-origins`
on the internal listener. Backfill checks public creator records against stored
media and prior hosting evidence; finding an object alone is not approval.
For a permanent station-wide withdrawal, `POST /withdraw-origin` there with
`{"cid":"<content CID>"}`. A success means the bucket marker is durable, not
that the PDS has acknowledged deletion. It cannot recall bytes already delivered.
An indexed moderation label suppresses playback locally but does not permanently
withdraw an existing attestation. Deleting an approved video retires that source;
the station can still host identical bytes for another approved video.

Station uploads resume on any node. After an ambiguous PATCH response, use
TUS `HEAD` before sending more bytes: its offset is the committed one.
Use S3 `AbortIncompleteMultipartUpload` for abandoned multipart requests.
Do not expire `stations/<namespace>/uploads/` by age: it also holds completed
objects. Reclaim unreferenced parts offline, with all writers fenced.

**Cutover.** First run the old node on the current schema. Finish or cancel
uploads, drain queued external side effects and recording finalizations, stop
all writers, and back up the old DB. Retain the original media objects; the
database migration does not copy files. Use a new namespace, with the S3
settings and key file supplied to the migration process:

```sh
streamplace migrate-statedb --from sqlite:///var/lib/streamplace/state.sqlite --to station://my-station
```

Inspect the per-table counts before starting a node. An import refuses a
nonempty station target rather than merge old sessions, resurrect deletions,
or replace signing keys. The bucket and external PDS are independent backup
domains; a bucket restore cannot roll back an OAuth refresh, a published
record, or a webhook delivered upstream. Fence old writers before any restore.
Do not restart the old DB as a writable primary after accepting new traffic.

**Offline maintenance.** After stopping and fencing every writer,
`streamplace --db-url station://my-station compact-station-db --to station://sql-backup`
copies only reachable operational SQL pages to a fresh namespace. It does
not delete old pages and is **not** a backup of the creator shards, recording
manifests, or media. Unreferenced pages may include encrypted old credentials;
use your normal bucket backup, retention, and access policies for them.

Named locks covering external side effects do not expire on a timer. If
a process dies while refreshing a PDS credential, fence that process and
reconcile the result with the PDS before releasing the lock. Never sweep the
lock prefix while nodes can write. A fresh login may be required after an
uncertain refresh.

Synchronize node clocks. A worker that loses its task claim cannot commit
another result, but it cannot recall a webhook or push message already in
flight. A `BLOCKED` task with an unconfirmed external delivery requires
reconciliation with that destination; do not blindly requeue it.

**Scale boundary.** Add playback nodes for egress; a viewer is not assigned
to the creator's writable shard. Each playback node still maintains its own
moderation and record index. Use `streamplace sync` before sending traffic
to a cold node. The relational compatibility store still serializes commits
at one station-wide head: it removes the SQL service, not the cost of shared
writes. Measure head contention and bucket latency before a production rollout.

### Reusing a former PDS hostname

A Streamplace node does not host the accounts from a PDS that previously ran at
the same hostname. If their DID documents still point there, repository reads
return HTTP 508 (Loop Detected) rather than repeatedly proxying back to the node.
Restore the PDS or update those accounts' PDS endpoints to make their records
available again.

Repository reads forwarded by Streamplace carry `X-Streamplace-Repo-Proxy`.
Reverse proxies must preserve this header: a node can serve its own repository
to a forwarded request, but will not forward that request again.

### Docker

Running Streamplace from a Docker image works great except for Docker
networking. Streamplace relies heavily on WebRTC for playback, which requires
large numbers of ephemeral UDP ports to work properly. So, we recommend using
host networking. So that command would look something like:

```shell
  docker run \
    --name streamplace \
    -d \
    -e SP_HTTP_ADDR=:80 \
    -e SP_HTTPS_ADDR=:443 \
    -e SP_SECURE=true \
    -e SP_BROADCASTER_HOST=example.com \
    -e SP_ALLOWED_STREAMS=did:web:example.com,did:plc:rbvrr34edl5ddpuwcubjiost \
    -v /var/lib/streamplace:/var/lib/streamplace \
    --net=host \
    ghcr.io/streamplace/streamplace
```

## Download a binary

Binaries for all platforms are available to download from
[our GitLab server](https://git.stream.place/streamplace/streamplace/-/releases).

## Single-user node with an upstream station

```shell
streamplace --http-addr=127.0.0.1:38080 \
  --upstream-station=did:web:stream.place \
  --relay-host=moqt://streamplace.network
```

`SP_UPSTREAM_STATION` and `SP_RELAY_HOST` are the environment equivalents.
Without a station, normal server indexing and relay selection remain unchanged.
With one, the node is a single-user AppView: it runs the usual indexing, but
only for the accounts logged into it.

- It takes each logged-in account's events from that account's per-DID track
  on every `--relay-host`, which must all be MoQ relays: an
  [atmoq](https://github.com/streamplace/atmoq) relay serves a per-DID track on
  demand. There is no aggregate firehose, no `bsky.network` and no local PDS
  firehose.
- It indexes only users with a successfully completed, non-revoked local OAuth
  grant. Pending logins, unrelated followed users and repos left over from a
  previous standalone run do not permit PDS scraping. Per-DID subscriptions
  start at login and end when the last eligible grant ends. Reconnects
  reconcile the same scoped users with their actual PDS; historical records
  remain on disk after logout.
- It signs users in through the station: the `#streamplace` HTTPS origin in
  the station's DID document. The local node delegates to the station using
  its own DPoP key; the station independently authenticates to the PDS using
  its public HTTPS callback. Local HTTP OAuth requires a loopback listener; no
  public callback or development-only OAuth flag is needed on the local node.
- It pushes the streams it ingests to the station over MoQ, unless `--peers`
  names other nodes.

Deploy the station's chained-OAuth support before releasing Desktop clients
that default to `did:web:stream.place`.
