const now = Date.now();
const response = http.post(
  `${CAPTION_API_URL}/xrpc/place.stream.caption.pushCaptions`,
  {
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${STREAM_KEY}`,
    },
    body: JSON.stringify({
      streamer: ACCOUNT_DID,
      language: "en",
      source: "human",
      cues: [
        {
          id: `maestro-${now}`,
          startTime: new Date(now).toISOString(),
          endTime: new Date(now + 4000).toISOString(),
          text: "Maestro live caption proof",
          final: true,
        },
      ],
    }),
  },
);
output.captionReady = response.status === 200;
output.captionTrackID = "";
if (!output.captionReady) {
  if (
    response.status !== 400 ||
    JSON.parse(response.body).message !== "StreamNotLive"
  )
    throw new Error(
      `pushCaptions returned ${response.status}: ${response.body}`,
    );
} else {
  const listed = http.get(
    `${CAPTION_API_URL}/xrpc/place.stream.caption.listTracks?streamer=${encodeURIComponent(ACCOUNT_DID)}`,
  );
  if (listed.status !== 200)
    throw new Error(`listTracks returned ${listed.status}: ${listed.body}`);
  const tracks = JSON.parse(listed.body).tracks;
  const human = tracks.find((track) => track.source === "human");
  output.captionReady = !!human;
  if (human) output.captionTrackID = human.id;
}
