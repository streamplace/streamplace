---
title: Captions for your stream
description: Enable captions, bring your own captioner, and manage VOD captions.
sidebar:
  order: 30
---

Captions let viewers follow your stream without hearing the audio. Streamplace
supports caption tracks on live HLS, low-latency WebRTC, and recorded videos
(VODs), on the web, iOS, and Android. Automatic recognition can make mistakes;
for important content, consider a human captioner or corrected VOD captions.

## What viewers get

Use the player's **CC** button to turn captions on or off. Open its caption menu
to choose a track by language; automatic tracks are marked as automatic. A track
must be available before it can be selected.
Without an explicit track choice, live players follow the canonical track with
the newest displayable cue, falling back to node sidecars when canonical speech
is absent. An explicit menu choice stays selected.

Open **Settings → Captions**, or **Caption style** in the player menu, to preview
and change text size, font, text color and opacity, text edges, and background
and window colors and opacity. These are viewer preferences, not changes to the
stream's caption text.

On iOS and Android, Streamplace's style controls apply to the live low-latency
caption overlay. Native HLS subtitles, including VOD playback, use your device's
accessibility caption settings instead. On the web, Streamplace renders caption
tracks with your selected style.

## Choose how your stream is captioned

In the live dashboard's stream settings, **Captions** is on by default:

- **On (`auto`)**: the origin node recognizes speech and masters captions into
  your signed stream. Your embedded or pushed captions take precedence when
  supplied. Recognition depends on available node resources.
- **Off (`off`)**: your canonical stream carries no caption track. This does
  **not** disable optional node captions while **Allow node captions** is on.

Under the advanced caption settings:

- **Use my own captions (`ingest`)**: supply CEA-608/708 captions embedded in your
  encoder's H.264 video, or push captions from a browser captioner, CART service,
  or external tool. The origin masters these into your signed stream without
  running automatic speech recognition.
- **Allow node captions**: on by default. Nodes may supply separate accessibility
  captions when your stream has no canonical captions. Turn this off to prevent
  nodes from generating or syndicating those sidecars; it does not remove your
  canonical captions.
- **Languages**: optionally enter up to four comma-separated BCP 47 tags, such
  as `en, es`, most prominent first. These are recognition hints, not a request
  to translate your stream. Leave them blank for automatic language detection.

The policy travels in your signed stream metadata so relay nodes see the same
preferences. See the [caption policy reference](/docs/lex-reference/metadata/place-stream-metadata-captionpolicy/)
for the protocol fields, and the [node operator guide](/docs/guides/installing/captions/)
for recognition resources and latency.

## Browser captioner and OBS display

There are **two separate pages**:

1. **Captioner**: `https://YOUR-NODE/captioner` listens to your microphone and
   runs speech recognition on your device. Sign in as the streamer, start your
   livestream, choose the microphone, language, and tiny/base/small model, then
   start captions. Only recognized caption text is sent to the node, not this
   page's microphone audio. Use its speed measurement before choosing a model;
   a larger model may not keep up with live speech. A positive calibration
   offset delays cue timestamps to match encoder latency.
   Final captions commit after 600 ms of silence or a 12-second window and are
   pushed about once a second.
2. **Display overlay**: add this URL as an OBS Browser Source:

   ```text
   https://YOUR-NODE/embed/captions/YOUR-DID-OR-HANDLE?fontSize=42&color=white&background=black&position=bottom&maxLines=3
   ```

   The display is transparent outside the caption box and needs neither login
   nor microphone permission. It receives the stream's published live captions;
   it does not recognize speech. Optional `track` selects a particular track.
   `fontSize` is in pixels, `color` and `background` are CSS colors, `position`
   is `top`, `center`, or `bottom`, and `maxLines` is 1–10. URL-encode colors
   containing special characters, such as `#`.
   The optional `font` uses a shared caption font name, such as
   `proportionalSans` or `monospacedSans`. Without `track`, the overlay follows
   the newest displayable canonical cue, or a sidecar when canonical speech is
   absent. It shows the newest cue on arrival for its duration (at least five
   seconds), until replaced. Player overlays instead use the segment
   presentation clock and cue start/end intervals; pausing freezes their
   fallback caption clock. HLS subtitles follow the playback element's timeline.

Use **ingest** mode if you want the browser captioner to be your only canonical
caption source. **Auto** also accepts its pushed captions, but may recognize
speech until your captions arrive. The captioner does not change this setting
for you.
Browser Go Live's **Caption my stream on this device** uses the outgoing audio
track and tiny model without changing policy. Enabling it may reload the page
for browser isolation; restart Go Live after that reload.

You can keep the captioner in an ordinary browser and use only the display in
OBS. To run the **captioner itself** in an OBS Browser Source, launch OBS with
`--enable-media-stream`, use an HTTPS URL, then use **Interact** to sign in,
grant microphone permission, and start captions. The display-only source does
not need this flag. Do not use `--use-fake-ui-for-media-stream` unless you intend
to grant automatic microphone access to every browser source. Disable **Shutdown
source when not visible** for sources that must stay active through scene changes.

An OBS display overlay becomes part of the video image when broadcast; viewers
cannot turn that burned-in text off with CC. You do not need an OBS display
source to give Streamplace viewers selectable captions.

## Push captions from CART or encoder tools

Send an `application/json` POST to your **origin node** at
`/xrpc/place.stream.caption.pushCaptions`. Authenticate with the streamer's
OAuth session, or with `Authorization: Bearer <stream key>` using an active,
registered Streamplace stream key. Treat the key as a secret. If `streamer` is
provided, it must be the authenticated streamer's DID. The stream must be live
on this node; otherwise the procedure returns `StreamNotLive`.

The API accepts at most **100 cues per request**, **64 UTF-8 bytes per cue ID**,
and **2,000 UTF-8 bytes of text per cue**, with a **2 MiB request-body limit**.
The complete batch is validated before publishing any cue. Send small batches
promptly rather than waiting to collect 100 cues. Use UTC wall-clock times
for when the words were spoken, not offsets
from zero or the captioner's HTTP send time. The end must be after the start.
Keep the captioner and encoder clocks synchronized.

For example, with your stream running at the example times:

```json
{
  "language": "en",
  "source": "human",
  "cues": [
    {
      "id": "cart-0001",
      "startTime": "2026-10-01T18:00:05.000Z",
      "endTime": "2026-10-01T18:00:07.500Z",
      "text": "Welcome to the stream.",
      "final": true
    }
  ]
}
```

`source` defaults to `human`; use `auto` for a recognizer. `final` defaults to
`true`. An interim cue (`final: false`) can be revised using the same cue ID.
Pushed cues become canonical under **auto** or **ingest**. With canonical
captions **off**, they are sidecars when node captions are allowed, or local to
this node's viewers when they are not.

See [pushCaptions](/docs/lex-reference/caption/place-stream-caption-pushcaptions/)
and [cue definitions](/docs/lex-reference/caption/place-stream-caption-defs/)
for the complete API. Authentication and clock handling are implemented in
[`place_stream_caption_push.go`](https://github.com/streamplace/streamplace/blob/main/pkg/spxrpc/place_stream_caption_push.go);
the limits are defined in
[`pushCaptions.json`](https://github.com/streamplace/streamplace/blob/main/lexicons/place/stream/caption/pushCaptions.json)
and [`defs.json`](https://github.com/streamplace/streamplace/blob/main/lexicons/place/stream/caption/defs.json).

## Upload and download VOD captions

Open one of your videos for editing and find **Captions**. Enter the caption
language (for example, `en` or `en-US`) and upload a **`.vtt` (WebVTT)** or
**`.srt` (SubRip)** file. Cue times must be relative to the start of the video.
You must be signed in as the video's owner. Importing again for the same video
and language replaces your earlier imported or human-authored transcript
records, rather than adding a duplicate copy.

Each listed track has **VTT** and **SRT** download actions. Downloads contain
caption text and timing, not burned-in video. For tools, use
[importCaptions](/docs/lex-reference/caption/place-stream-caption-importcaptions/),
[listTracks](/docs/lex-reference/caption/place-stream-caption-listtracks/), and
[getCaptions](/docs/lex-reference/caption/place-stream-caption-getcaptions/).

## Where transcripts live

Canonical live captions and your VOD imports are published as
[`place.stream.caption.transcript`](/docs/lex-reference/caption/place-stream-caption-transcript/)
records in **your AT Protocol repository**. The origin uses your stored OAuth
session to write canonical transcripts; a stream key alone is not an OAuth
session for publishing records. Canonical text is also archived inside the
signed stream's MUXL text tracks.

Node-generated sidecars are separate: their transcript records live in the
**generating node's server repository**, not yours. Forwarding a sidecar does
not make the relay its author. This attribution is implemented by
[`RepoPublisher`](https://github.com/streamplace/streamplace/blob/main/pkg/captions/records/publisher.go).
For archival formats and playback integration, see
[the protocol notes](/docs/features-dev/captions/).
