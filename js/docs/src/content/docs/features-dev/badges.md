---
title: badges system
description: user badges for chat messages
---

## Overview

Badges appear next to usernames in chat messages. they're small icons that indicate status (streamer, mod, vip, etc.). There will be max 3 badges shown at once. One of the badges is server-based (e.g. streamer, mod, node staff badge), but the other two can be selected from a pool of cosmetic badges (such as subscription badges, event badges et al.). These cosmetic badges are cryptographically signed by the issuing party, and all the user needs to do is apply them to their chat profile. Note that certain badges may appear/disappear based on the current streamer's chat tktk.

## Lexicon schemas

We have three relevant lexicons.

1. **`place.stream.badge.defs`** - badge definitions and view model

   - defines known badge types: `mod`, `streamer`, `vip`
   - `badgeView` object: `{badgeType, issuer, recipient, signature?}`

2. **`place.stream.badge.issuance`** - record of badge grant

   - stored as atproto record (key: tid)
   - issued by streamer or other authorized entity
   - example: streamer issues vip badge to a user

3. **`place.stream.badge.display`** - user's badge selection
   - user-controlled record defining which badges to show
   - array of up to 3 `badgeSelection` objects
   - first slot server-controlled (mod/streamer/staff), second slot is streamer-specific (vip, subscription), third slot is user-set (event, staff2, node subscription, etc.)

:::note
This may get changed to be in the user's chat profile? Maybe we could have a "main" chat profile and a streamer-specific profile?
:::

## TODO

- [ ] implement cryptographic signatures for badge issuance
- [ ] implement badge issuance ui (streamer grants vip badges)
- [ ] implement badge selection ui (users choose which badges to display)
- [ ] add more badge types (subscriber, founder, staff, etc)
