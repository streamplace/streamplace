
# Streamplace Compositor

We at Streamplace pride ourselves on being the only ones stupid enough to try very bad ideas that
end up working really well. [Maxim 14: "Mad Science" means never stopping to ask "what's the worst thing that could happen?"](http://schlockmercenary.wikia.com/wiki/The_Seventy_Maxims_of_Maximally_Effective_Mercenaries)

Anyway, here's a project that uses Chromium (via [nw.js](https://github.com/nwjs/nw.js/)) in a
Docker container for the purposes of compositing live video using WebGL.

## Usage

Required environment variables:
`URL`: Video-containing page that I should load.
`SELECTOR`: Selector, passed to `document.querySelector`, of the `<canvas>` element you'd like to
render into video.

## Requirements

For production: I have no idea, it works on my Ubuntu Kubernetes cluster on a computer under my
desk. I'll check back here when I know more.

For development: You'll want `nw` and `socat` and `ffmpeg` on your PATH. Your FFMpeg should
support `libx264`, though it'd be kind of nice to have a WebM option as well.

## Limitations

### Security

To get this thing working, we had to flip off a couple Chromium security features -- to wit,
`--no-sandbox` and `--ignore-gpu-blacklist`. Combine this with the fact that it's a
not-that-vetted `nw.js` project and omg problems.

That's all by way of saying: please oh please only point this thing at websites that you trust. If
you just want a demo, [https://streamplace.github.io/demo-video/](https://streamplace.github.io/demo-video/) is a great test video.

### Video

30fps for now. Deal with it.

This has only been tested with 1920x1080 video, though I don't anticipate any problems with other
aspect ratios.
