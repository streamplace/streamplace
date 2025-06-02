---
title: Discord Webhooks
description: Configure Discord webhooks for livestream announcements and chat
sidebar:
  order: 30
---

Streamplace has basic Discord webhook integration if you're running your own
node — guide coming soon! You can send livestream notifications and chat
messages to a Discord channel. Basic usage looks like this:

```shell
streamplace --discord-webhooks '[
  {
    "url": "https://discord.com/api/webhooks/server-id/webhook-secret",
    "type": "chat",
    "did": "did:plc:rbvrr34edl5ddpuwcubjiost",
    "prefix": "streamplace: "
  },
  {
    "url": "https://discord.com/api/webhooks/server-id/webhook-secret",
    "type": "livestream",
    "did": "did:web:stream.place",
    "rewrite": [{ "from": "#gaming", "to": "<@&1378853638738411641>" }],
    "suffix": " @everyone"
  }
]'
```

<!-- https://github.com/streamplace/streamplace/issues/245 -->

| Field     | Description                                                                                                                                                                                                                                                                                                                                            |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `url`     | Your Discord webhook URL.                                                                                                                                                                                                                                                                                                                              |
| `type`    | Should be `chat` for a chat webhook or `livestream` for a livestream notification webhook.                                                                                                                                                                                                                                                             |
| `did`     | The DID of the AT Protocol (Bluesky) account of the streamer. If you don't have yours, you can look it up on [pdsls](https://pdsls.dev/).                                                                                                                                                                                                              |
| `prefix`  | A prefix appended to every message. For example, if you're amalgamating chat from several sources, you might want a prefix of `"streamplace: "` before every chat message.                                                                                                                                                                             |
| `suffix`  | A suffix appended to every message. For example, you might want your live notifications to ping everyone in your Discord server, so you could use a suffix of `" @everyone"`.                                                                                                                                                                          |
| `rewrite` | Rewrite rules for the message before it hits Discord. For example, if you want your `#gaming` streams to ping the `@gaming stream enjoyers` role in your Discord server, you can [find the Role ID of the `@gaming stream enjoyers` role](https://discordhelp.net/role-id) and rewrite with `[{ "from": "#gaming", "to": "<@&1378853638738411641>" }]` |
