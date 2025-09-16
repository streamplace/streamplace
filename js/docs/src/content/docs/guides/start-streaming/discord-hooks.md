---
title: Discord Webhooks
description: Configure Discord webhooks for livestream announcements and chat
sidebar:
  order: 30
---

Streamplace supports Discord webhook integration for receiving livestream
notifications and chat messages. You can create, manage, and configure webhooks
to customize how events are delivered to your Discord channels.

## Webhook Events

You can configure webhooks to listen for specific events. For right now, the
following events are supported:

- `Chat`: Triggered when a chat message is sent.
- `Livestream`: Triggered when a livestream starts.

## Creating a Webhook

To create a webhook, go to the "Settings" page of the Streamplace web app, then
navigate to the "Webhooks" section. Click on "Create Webhook". The following
fields are required:

- Name: Webhook URL. For example,
  `https://discord.com/api/webhooks/{webhook.id}/{webhook.token}`
- Events: Select the events you want to subscribe to (e.g., `Chat Messages`,
  `Livestream Started`). `Livestream Started` is pre-checked by default.

We'd recommend also filling out these optional fields:

- Name: A name for the webhook (e.g., "Discord Livestream Notifications") that
  you can remember.
- Description: A description of what this webhook is for (e.g., "Sends
  livestream start notifications to Discord channel").
- Prefix: A prefix to add to each message sent by this webhook (e.g.,
  "[Streamplace] "). Will apply to both Chat and Livestream events!
- Suffix: A suffix to add to each message sent by this webhook (e.g., "is now
  live!"). Will apply to both Chat and Livestream events!
- Text replacements: A list of text replacements to apply to chat messages sent
  by this webhook. Each replacement consists of a "from" string and a "to"
  string. For example, you could replace all instances of "foo" with "bar".

After filling out the form, click "Create" to save your webhook. You should see
it listed in the "Webhooks" section.

## Updating a Webhook

To update a webhook, go to the "Settings" page of the Streamplace web app, then
navigate to the "Webhooks" section. Find the webhook you want to update and
click on the "pen" icon next to it. This will open the webhook edit form, where
you can modify the fields as needed. After making your changes, click "Update"
to save your changes.

## Deleting a Webhook

To delete a webhook, go to the "Settings" page of the Streamplace web app, then
navigate to the "Webhooks" section. Find the webhook you want to delete and
click on the "trash" icon next to it. A confirmation dialog will appear; click
"Delete" to confirm. The webhook will be removed from the list.

## Recommendations

We'd recommend:

- Creating separate Discord channels for livestream notifications and chat
  messages to keep them organized.
  - If you want to have one webhook for both chat and livestream events, you can
    create multiple webhooks with the same URL but different event subscriptions
    and prefixes/suffixes/replacements.
- Testing your webhook by starting a livestream or sending a chat message to
  ensure that notifications are being sent correctly.

## API Documentation

See these endpoint pages:

- [Create Webhook](/docs/api/operations/placestreamservercreatewebhook)
- [Get Webhook](/docs/api/operations/placestreamservergetwebhook)
- [List Webhooks](/docs/api/operations/placestreamserverlistwebhooks)
- [Update Webhook](/docs/api/operations/placestreamserverupdatewebhook)
- [Delete Webhook](/docs/api/operations/placestreamserverdeletewebhook)
