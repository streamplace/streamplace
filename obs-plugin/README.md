# obs-streamplace

An OBS Studio plugin (OBS 30.2 or newer) that streams to a Streamplace node as
fragmented MP4 over HTTP. The node configures the stream through the same
request OBS sends for Enhanced RTMP multitrack video, so it can choose how many
video renditions and audio tracks the plugin encodes.

## Using it

Extract the archive for your platform into OBS's plugin directory:

| Platform | Directory                                           |
| -------- | --------------------------------------------------- |
| Linux    | `~/.config/obs-studio/plugins/`                     |
| Windows  | `C:\ProgramData\obs-studio\plugins\`                |
| macOS    | `~/Library/Application Support/obs-studio/plugins/` |

Restart OBS, then pick **Streamplace** in Settings → Stream, the **Local
Streamplace node** server (`http://127.0.0.1:38080`), and your stream key. OBS
offers no free-form server field for listed services, so to use another
address, set `server` in the profile's `service.json`. The plugin only speaks
plain `http://`, and expects a node on the same machine.

OBS's Settings page only lists services from `rtmp-services`, so on startup the
plugin adds a Streamplace entry to the `services.json` copy in OBS's config
directory (`plugin_config/rtmp-services/`). The entry names the plugin's output
protocol, so OBS hides it whenever the plugin is not loaded. If OBS's service
updater replaces that file, the entry comes back on the next start.

## Protocol

When you start streaming, the plugin does three things in order.

1. It sends `POST {server}/api/ingest/client-configuration` with a JSON body in
   the shape of OBS's
   [GoLiveApi `PostData`](https://github.com/obsproject/obs-studio/blob/master/frontend/utility/models/multitrack-video.hpp).
   `authentication` carries the stream key, and `client.supported_codecs` is
   `["h264"]`.
2. It reads the response as a GoLiveApi `Config`:
   - `status.result: "error"` stops the stream and shows `status.html_en_us`.
   - `ingest_endpoints`: the plugin uses the first entry whose `protocol` is
     `FMP4`, replacing `{stream_key}` in its `url_template`.
   - `encoder_configurations` replaces the encoders from OBS's output settings
     with one video encoder per entry (H.264 encoder types only). Each entry
     becomes one video track.
   - `audio_configurations.live` creates one audio encoder per entry. `codec`
     is `aac` or `opus`, and `track_id` is the zero-based OBS audio mixer track
     to encode.
   - If either list is empty, the plugin uses OBS's own encoders for that kind
     of track.
3. It sends `POST` to the ingest URL with `Transfer-Encoding: chunked` and a
   `video/mp4` body:
   - First comes an init segment (`ftyp` + `moov`) with every video track,
     then every audio track.
   - After that it sends one `moof` + `mdat` fragment per frame of the first
     video track. Each fragment holds every sample completed since the last one.
   - Sample durations come from the real gaps between timestamps, so frames
     that OBS drops show up as longer samples.
   - The plugin ends the body when streaming stops.

## Building

From the repository root, in the builder container (see the
`streamplace-docker` skill):

```sh
make obs-plugin-linux-amd64   # or linux-arm64, windows-amd64, darwin-amd64, darwin-arm64
```

The archive is written to `bin/`. The build only needs the libobs headers. A
Meson wrap downloads them, pinned to OBS 30.2.3, the oldest release the plugin
supports. On Windows, the build generates an import library for `obs.dll` from
`obs.def`. When the plugin starts calling a new libobs function, add it there.
