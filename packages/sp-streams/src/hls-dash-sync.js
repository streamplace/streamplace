import xml2js from "xml2js";

// xml2js has the weirdest api
const parseXml = str => {
  let out;
  xml2js.parseString(str, function(err, result) {
    out = result;
  });
  return out;
};

/**
 * Fixes some faulty fields in our HLS manifests given our DASH manifests.
 */
export function hlsFix({ hls, dash }) {
  const xml = parseXml(dash);
  const set = xml.MPD.Period[0].AdaptationSet.find(
    set => set.$.contentType === "video"
  );
  const videoBandwidth = set.Representation[0].$.bandwidth;
  const hlsLines = hls.split("\n").map(line => {
    if (!line.includes("RESOLUTION")) {
      return line;
    }
    return line.replace(/BANDWIDTH=\d+/, `BANDWIDTH=${videoBandwidth}`);
  });
  return hlsLines.join("\n");
}
