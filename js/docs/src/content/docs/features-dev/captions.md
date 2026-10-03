---
title: Caption protocol and archival format
description: MUXL text tracks, compact transcript records, and live caption delivery.
---

Captions have two complementary representations: canonical WebVTT text tracks
in signed MUXL media, and compact AT Protocol transcript records for discovery,
imports, and timed text access. See the
[streamer guide](/docs/guides/start-streaming/captions/) and
[node operator guide](/docs/guides/installing/captions/) for configuration.

## Canonical MUXL text tracks

Canonical captions are **WebVTT text tracks inside the streamer-signed MUXL
segments**, not a separate Go-only archive. The origin attaches text before
signing. All nodes, including the origin, derive public canonical cues from the
text tracks of validated segments. HLS, websocket, and WebRTC outputs consume
those extracted cues rather than a parallel pre-signing canonical feed.

The MUXL text-track metadata convention is:

- **`TextTrack.Language`**: the BCP 47 tag, or `und` for an unknown language.
- **`TextTrack.Label`**: the source, `auto`, `ingest`, or `human`. Readers treat
  an empty or unrecognized label as `ingest`; it is not a free-form display name.

Tracks are declared when their first cues arrive and retain stable IDs and
immutable configuration during the ingest session. A declared track appears in
every later GoP; a track replaced by another source continues with empty text
segments rather than changing its metadata.
Cue pieces crossing GoP boundaries retain their identity and are clipped to the
corresponding GoP ranges. A track's cues never overlap: a cue that arrives
after its GoP was signed starts in the first unsigned GoP, after the previous
cue has had its own duration, and keeps its whole duration; the next cue on
the track ends it. Automatic captions stay up to 3 s past their last word until
replaced, so the track reads continuously between recognized phrases.

Live segments are never held for speech recognition, so their late words take
that path. The **recorded** copy of each origin segment (the S3 live recording
and the VODs finalized from it) is laid out a second time, once recognition
covers the GoP or `--captions-master-delay` passes. It is the live segment with
only its text runs replaced by runs signed with the streamer's key for the same
GoP span and `dc:date`, in ascending track-ID order. Audio, video, and the
node's transcoded audio run are byte-identical, so their signatures and the
transcode's source binding still verify. Isolated ingest workers send these
text runs to the main process as `ingestframe.Captions` frames before `End`.
Origin text tracks start at reserved numeric ID 100, above node-added AV
renditions. Continuous audio completion decodes only AV tracks and inserts its
audio run before any text tracks; late text declarations remain in signed
source bytes and completed archival segments.
Every policy stamps GoPs on the media clock, anchored to the first fragment's
arrival, not signing time; drift over one second reanchors the next GoP.
Pushed wall-clock cues use the inverse mapping.

MUXL cue times are absolute **media-timeline milliseconds**, not Unix time.
For live delivery the conversion is:

```text
cue wall-clock = signed segment startTime
               + (cue media time - reference AV decode time)
```

Never substitute a relay's receive time. Stored videos retain text-track
metadata and byte ranges; finalized VODs publish `place.stream.media.track`
records for text tracks alongside their audio/video tracks. VOD lookup reads
MUXL text cues on the video's AV timeline and merges them with indexed
transcript records. A matching transcript copy of the streamer's mastered text
is deduplicated; imports and differently authored tracks remain available.
Indexes retain each segment's language/source and explicit containing-GoP
reference clock. Reused numeric IDs after reconnect cannot relabel old captions.
Archives must use ascending numeric track-ID order within each GoP; the indexer
rejects noncanonical ordering rather than rewriting signed bytes. Track records
are published by `publishDraft`, not while the video remains a draft.

The convention and time conversion are implemented in
[`pkg/captions/muxl.go`](https://github.com/streamplace/streamplace/blob/main/pkg/captions/muxl.go);
origin attachment is in
[`pkg/media/captions_master.go`](https://github.com/streamplace/streamplace/blob/main/pkg/media/captions_master.go).
The ingest audio decoder accepts fMP4 runs with explicit `trun.data_offset` and
an explicit or default-moof `traf` base; unsupported implicit subsequent-traf
addressing is rejected. Sample counts must fit their encoded entries and paired
`mdat`, rather than trusting declared counts. Embedded caption decoding discards
incomplete CEA-708 packets; CEA-608 end-of-caption swaps memories without clearing
them (erase-nondisplayed-memory clears the back buffer).

## Compact transcript records

[`place.stream.caption.transcript`](/docs/lex-reference/caption/place-stream-caption-transcript/)
is a chunked record format adapted from Ionosphere's
`tv.ionosphere.transcript` encoding:

- **`startMs`** locates the first word relative to the subject's media start.
- **`text`** contains whitespace-separated words.
- **`timings`** is flat: each positive integer is the duration in milliseconds
  of the next word; a negative integer is a silence gap before the next word.
  Without a gap, a word begins where the previous one ended.

For example, `startMs: 1000`, `text: "Hello world"`, and
`timings: [300, -200, 400]` place “Hello” at 1000–1300 ms and “world” at
1500–1900 ms.

For a livestream subject, all chunks repeat **`mediaStart`**, the wall-clock
start of the session's first segment. `startMs` is relative to that instant.
For a video subject, `mediaStart` is absent and offsets are relative to video
start. A strong reference in `subject` identifies the livestream or video.
Use the generated reference for the remaining fields and record limits rather
than assuming a transcript fits in one record.

Canonical records are published by the origin in the **streamer's repository**
through their stored OAuth session. Imported VOD captions also live there.
Node-generated sidecars are published in the **generating node's server
repository**, with node attribution preserved through relays. Republishing a
received sidecar as the receiving node's transcript would lose that attribution.
For retry, overflow reconciliation, and shutdown guarantees, see
[transcript delivery](/docs/guides/installing/captions/#transcript-delivery-and-shutdown).

Imported SRT/WebVTT and human records preserve long cues, adjacent boundaries,
and authored line breaks. Their cues split at explicit silence gaps; imports
encode a 1 ms gap between otherwise adjacent cues by shortening the preceding
final word (or moving the next start 1 ms if that word is already 1 ms).
Automatic and ingest transcripts retain broadcast display layout. VTT, SRT,
JSON, VOD HLS, and overlays share the same record provider.
Live and transcript automatic/ingest cues share greedy word-boundary line fitting;
only a single overlong word can exceed the line limit. WebVTT timing settings
are ignored on import. SRT downloads write raw text; imports preserve entities
literally and strip only `<i>`, `<b>`, `<u>`, and `<font …>` formatting tags.
Literal text shaped like those formatting tags is treated as formatting.
WebVTT instead escapes text on export and decodes it on import.

The encoding is implemented in
[`pkg/captions/transcript/transcript.go`](https://github.com/streamplace/streamplace/blob/main/pkg/captions/transcript/transcript.go),
and repository publication in
[`pkg/captions/records/publisher.go`](https://github.com/streamplace/streamplace/blob/main/pkg/captions/records/publisher.go).

## Playback delivery

### HLS live and VOD

Each available track has an **`EXT-X-MEDIA:TYPE=SUBTITLES`** rendition in the
master playlist, using **`GROUP-ID="cc"`**. Every video variant references that
group with **`SUBTITLES="cc"`**. Subtitle playlists serve plain **WebVTT text
files**, aligned to video segment ranges, rather than putting MP4 text-track
bytes into a `.vtt` response. Each segment carries an **`X-TIMESTAMP-MAP`**
that aligns the local cue clock to the video's 90 kHz MPEG timestamp clock.
Cues overlapping adjacent segments may appear in both documents.

See
[`pkg/livehls/subtitles.go`](https://github.com/streamplace/streamplace/blob/main/pkg/livehls/subtitles.go)
and [`place_stream_playback_captions.go`](https://github.com/streamplace/streamplace/blob/main/pkg/spxrpc/place_stream_playback_captions.go).

### Livestream websocket

**`/api/websocket/<streamer DID>`** delivers JSON messages with
**`$type: "place.stream.caption.defs#liveCue"`**. A cue carries its track,
wall-clock `startTime`/`endTime`, text, ID, and `final` state. Clients replace
an earlier cue with the same ID rather than displaying it twice. Interim text
can be revised; canonical GoP pieces are coalesced into the same cue.
Final text and start time are immutable. A matching canonical final with the
same track and cue ID may extend its end time monotonically across GoPs;
clients must accept that continuation without accepting late interim or text
revisions.

New connections receive recent final cues (roughly the last ten seconds),
not the complete transcript. Public websocket captions are gated by the
stream's published-live state; unpublished preview captions are not a public
backlog. See
[`pkg/api/websocket.go`](https://github.com/streamplace/streamplace/blob/main/pkg/api/websocket.go)
and [`pkg/captions/livecue/livecue.go`](https://github.com/streamplace/streamplace/blob/main/pkg/captions/livecue/livecue.go).

### WebRTC / WHEP

WHEP sessions use a data channel labeled **`captions`**, carrying the same
`liveCue` JSON as the websocket. The viewer's SDP offer must negotiate a data
channel (`m=application`); an AV-only offer does not acquire a caption channel.
The server opens the channel and can also serve a viewer-opened channel with
that label. Public viewers receive only published-live cues; owner preview
access follows the WHEP session's viewer identity.

Fallback player caption clocks freeze only on an explicit pause. Connection
statuses such as starting, waiting, or stalled do not themselves mean the viewer
paused playback and must not prevent incoming live cues from being presented.

See
[`pkg/media/webrtc_captions.go`](https://github.com/streamplace/streamplace/blob/main/pkg/media/webrtc_captions.go).

### Node-to-node sidecars

Sidecars do not modify the streamer's signed media. On the segment-replication
websocket, `captions=1` opts into JSON text frames carrying
`place.stream.caption.sidecar#event`, alongside binary media frames. Relays
preserve the original node author and signed segment-clock timing, and honor
`allowNodeCaptions` from validated stream metadata before distribution.
Receivers derive track IDs from origin/source/language and bind authors to the
connected upstream's server DID. Replay before the first media segment stays
private until its signed policy is validated. Incoming canonical tracks remove
competing sidecars and stop recognition. The hub rejects cross-origin ID
collisions, stores only finals, and bounds track count/history; viewer delivery
remains nonblocking. Websocket replication is the implemented origin pull path;
Iroh currently has no segment transport.

## API references

Use the generated lexicon pages as the schema source of truth:

- [Caption policy](/docs/lex-reference/metadata/place-stream-metadata-captionpolicy/)
- [Track views and live/pushed cues](/docs/lex-reference/caption/place-stream-caption-defs/)
- [Transcript record](/docs/lex-reference/caption/place-stream-caption-transcript/)
- [List tracks](/docs/lex-reference/caption/place-stream-caption-listtracks/)
- [Download captions](/docs/lex-reference/caption/place-stream-caption-getcaptions/)
- [Push live captions](/docs/lex-reference/caption/place-stream-caption-pushcaptions/)
- [Import VOD captions](/docs/lex-reference/caption/place-stream-caption-importcaptions/)

`pushCaptions` evaluates one server timestamp per request. Every cue's start must
be within the **last five minutes** and **at most 30 seconds ahead** of it
(inclusive), with duration **greater than zero and at most 30 seconds**. Any
invalid cue rejects the entire batch atomically with HTTP 400.

Viewer reducers independently retain at most **32 cues per track**, newest
starts first, and drop cues starting more than **30 seconds ahead of the segment
presentation clock** (local arrival time until that clock is available).

Players keep a track's newest cue up for **2 seconds** past its end while no
later cue is known: a canonical cue's end is provisional until the next
segment's text arrives. Recognized (`auto`) speech rolls up, as in broadcast
roll-up captions: consecutive cues run together in rows of 42 columns (scaled
down for larger caption sizes), each sentence starting a row. A player shows
every row of the newest cue and at least the last two rows, and keeps them up
until 2 seconds after the newest cue ends. Other sources keep their cue lines.

`getCaptions` supports `vtt`, `srt`, and JSON cues. Video offsets count from
video start; live downloads cover the node's current window, with JSON's
`epoch` identifying its time base. `listTracks` takes a live streamer or a video
AT-URI; use the returned track IDs instead of inventing IDs from language names.
