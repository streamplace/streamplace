---
title: Access control
description: Decide who can view, stream to, upload to, and be syndicated by your node.
---

Every Streamplace node has a small role-based access policy. Admins edit it
from **Settings → Access** in the app; the environment variables you may
already be using keep working as seeds.

## Roles

| Role        | What it allows                                                                  |
| ----------- | ------------------------------------------------------------------------------- |
| `admin`     | Manage branding and access control. Admins hold every other role automatically. |
| `viewer`    | Use the frontend and playback. Only matters when the viewer mode is not `open`. |
| `streamer`  | Ingest live media to this node.                                                 |
| `syndicate` | Have this account's media carried from other nodes onto this one.               |
| `vod`       | Upload videos and have livestreams recorded.                                    |

## Modes

Each role has a mode:

- **open** — everyone holds the role, including anonymous visitors.
- **allowlist** — only accounts with a grant hold the role.
- **off** — nobody holds the role. Admins are always exempt.

The `admin` role is always `allowlist`.

## A private node

To bring up a node that nobody can use until you let them in, start it with
your own DID as admin and the viewer role seeded to `allowlist`:

```bash
SP_ADMIN_DIDS=did:plc:yourdid SP_ACCESS_POLICY=viewer=allowlist streamplace
```

Sign in as that admin, open **Settings → Access**, and grant `viewer` to the
accounts that should get in. Admins never need a viewer grant.

Set the `viewer` role to `allowlist` and the node answers nothing but the
sign-in flow to anyone who is not on the list. That includes the API, link
cards, playback, thumbnails and chat. Branding stays public so the sign-in
wall shows the node's own logo and name. Visitors see a
"this node is private" wall, and signed-in accounts that are not on the list
see "you're not on the list". Nodes that share a service key with this one
(the same station) are not affected.

Grant `viewer` to the accounts that should get in. Admins never need a
viewer grant.

## Environment seeds

The variables below seed grants at startup. They show up in the Access screen
marked "from environment" and cannot be revoked there; remove them from the
environment instead. Grants you add in the app live in the node's state
database and survive restarts.

| Variable                 | Seeds                                                                                            |
| ------------------------ | ------------------------------------------------------------------------------------------------ |
| `SP_ADMIN_DIDS`          | `admin` grants. You need at least one to reach the Access screen the first time.                 |
| `SP_ACCESS_POLICY`       | Initial modes as `role=mode` pairs, e.g. `viewer=allowlist,vod=off`. A mode set in the app wins. |
| `SP_ALLOWED_STREAMS`     | `streamer` grants. When set, the streamer mode defaults to `allowlist`; when empty, to `open`.   |
| `SP_SYNDICATE`           | `syndicate` grants. `*` makes the default mode `open`; empty makes it `off`.                     |
| `SP_WIDE_OPEN`           | Forces viewer, streamer and vod to `open`. Development only.                                     |
| `SP_DISABLE_SYNDICATION` | Forces syndicate to `off` in both directions.                                                    |
| `SP_BETA_INVITE_DID`     | Accounts holding a `place.stream.beta.invite` for `vod` from this DID also hold the `vod` role.  |

When no vod mode has been set explicitly and no invite issuer is configured,
anyone who may stream may also upload, which is what older nodes did.

## Where the data lives

Grants and the policy are modeled as records in an atproto space owned by
the node's broadcaster DID, addressed as
`at://{broadcaster}/space/place.stream.access.control/self/{admin}/place.stream.access.grant/{rkey}`.
Until the atproto spaces implementation ships they are stored in the node's
state database under those URIs, so they can move into a real space later
without changing their shape. The API lives under `place.stream.access.*`.
