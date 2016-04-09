mpeg-munger
===========

This project provides a node transform stream that rewrites the timestamps (PTS and DTS) of a
MPEGTS multimedia stream as quickly as it can.

It is not a full Node parser for MPEGTS. For that, check out the excellent
[RReverser/mpegts](https://github.com/RReverser/mpegts) project. This project, by contrast, only
reads and writes to the tiny subset of the stream necessary to rewrite the PTS and DTS.

Its primary use is to bring video streams from multiple sources into sync with each other no
matter what sort of timestamps they use.
