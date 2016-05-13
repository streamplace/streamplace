
import path from "path";
import fs from "fs";

import S3ConcatStream from "../apps/pipeland/src/classes/S3ConcatStream.js";

const region = "us-west-2";
const bucket = "userfiles.stream.kitchen";
const prefix = "test/2016/05/13/2016-05-13-05-57-10-437-BattleBornFile-default-audio";

const s3Stream = new S3ConcatStream({region, bucket, prefix});
const writeStream = fs.createWriteStream(`${path.basename(prefix)}.ts`);
s3Stream.pipe(writeStream);
