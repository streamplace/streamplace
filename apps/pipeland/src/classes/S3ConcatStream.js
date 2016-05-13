
import {Readable} from "stream";
import AWS from "aws-sdk";

export default class S3ConcatStream extends Readable {
  constructor({bucket, prefix, region, accessKeyId, secretAccessKey}) {
    super();
    this.bucket = bucket;
    this.prefix = prefix;
    this.s3 = new AWS.S3({accessKeyId, secretAccessKey, region});
    this.started = false;
  }

  _nextKey() {
    if (this.keys.length === 0) {
      // Stop!
      this.push(null);
      return;
    }
    const key = this.keys.pop();
    // console.log(`Downloading ${key}`);
    this.s3.getObject({Key: key, Bucket: this.bucket})
      .on("httpData", (chunk) => {
        this.push(chunk);
      })
      .on("error", (err) => {
        // console.log(err);
      })
      .on("complete", () => {
        this._nextKey();
      })
      .send();
  }

  _read() {
    if (this.started === true) {
      return;
    }
    this.started = true;
    this.s3.listObjectsV2({Bucket: this.bucket, Prefix: this.prefix}, (err, data) => {
      if (err) {
        // console.log(err.stack);
        throw err;
      }
      this.keys = data.Contents.map(file => file.Key).sort().reverse();
      this._nextKey();
    });
  }
}
