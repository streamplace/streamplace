import { Client as MinioClient } from "@streamplace/minio";
import url from "url";
import mpegMungerStream from "./mpeg-munger-stream";
import debug from "debug";

const log = debug("sp:file-output-stream");

/**
 * Implements the file API described in sp-plugin-core's File.yaml. UUID prefixes are concatenated
 * in ASCII order. This lets us split up mpegts files however we like.
 */
export default function(params) {
  const { accessKeyId, secretAccessKey, host, bucket, prefix } = params;
  const parsed = url.parse(host);
  const secure = parsed.protocol === "https:";
  const mpegMunger = mpegMungerStream();
  let port = parsed.port;
  if (!port) {
    port = secure ? 443 : 80;
  } else {
    port = parseInt(port);
  }
  const minio = new MinioClient({
    endPoint: parsed.hostname,
    accessKey: accessKeyId,
    secretKey: secretAccessKey,
    secure: secure,
    port: port
  });
  minio
    .putObject(bucket, prefix, mpegMunger)
    .then(etag => {
      log("Uploaded", etag);
    })
    .catch(err => {
      throw err;
    });
  return mpegMunger;
}
