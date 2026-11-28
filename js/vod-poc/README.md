# VOD time-to-first-frame proof of concept

A watch page with none of `js/app` or `js/web`: one HTML file per video that
paints its first frame from its own bytes, then fetches the rest over Media
over QUIC and switches up to a better rendition.

```sh
# Inside the builder, after make dev:
hack/vod-poc-local.sh            # the Playwright flow and vod-ttff against it
hack/vod-poc-local.sh --serve    # print the two watch URLs and keep serving
```

## How a page loads

1. `prepare-media.sh` makes two H.264 renditions with ffmpeg: a 240p starter
   at 150 kbps, whose keyframes are about 2 KB, and 1080p at 4 Mbps. Both have
   a keyframe every second, so groups line up.
2. `streamplace vod-poc` (`pkg/cmd/vodpoc.go`) publishes both renditions as
   finished tracks on a MoQ broadcast, with the node's own MoQ library, and
   serves each fetched group on demand (`TrackProducer.Dynamic`): the library
   drops groups idle for 30 s or more. It also bakes `index.html` for each
   video, inlining:
   - the manifest: renditions, decoder configs, the MoQ URL and certificate
     hash;
   - the starter's first keyframe, in its own script;
   - the rest of the starter's first second, in the next script.
3. The inline engine opens a WebTransport session immediately, decodes the
   keyframe with WebCodecs, and draws it on a canvas. Nothing else has been
   requested yet: the first frame needs only the HTML's first 10 KB (5.5 KB
   gzipped, inside a first TCP congestion window).
4. `src/player.ts`, bundled with the pinned `@moq/net`, is imported once the
   first frame is on screen, so even a cached copy can't compile ahead of it.
   It takes the WebTransport session over and fetches later
   groups by sequence, two seconds ahead of playback. Until a 1080p group
   arrives before playback needs it, it fetches the starter's copy too and
   uses that instead; after one does, it stays on 1080p.

## Left out

Audio, seeking, `?t=`, downswitching, Safari (no WebTransport pinning) and
anything but Chromium-family WebCodecs are out of scope. The MoQ certificate
is self-signed and pinned by hash, so the page is baked when the server
starts; a real deployment would bake at publish time and serve MoQ with a
publicly trusted certificate.
