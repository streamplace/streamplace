/**
 * S3 upload utilities.
 *
 * Flow:
 * 1. Stream archive chunks to a temp key via multipart upload
 * 2. Feed each chunk through BLAKE3 to compute the CID
 * 3. Multipart-copy temp → {cid}.mp4
 * 4. Delete temp
 * 5. PutObject init segment blobs + metadata JSON
 */

import {
  CompleteMultipartUploadCommand,
  CreateMultipartUploadCommand,
  DeleteObjectCommand,
  HeadBucketCommand,
  PutObjectCommand,
  S3Client,
  UploadPartCommand,
  UploadPartCopyCommand,
} from "@aws-sdk/client-s3";

export interface S3Config {
  endpoint: string;
  bucket: string;
  accessKeyId: string;
  secretAccessKey: string;
}

export function createS3Client(config: S3Config): S3Client {
  return new S3Client({
    endpoint: config.endpoint,
    region: "auto",
    credentials: {
      accessKeyId: config.accessKeyId,
      secretAccessKey: config.secretAccessKey,
    },
    forcePathStyle: true,
  });
}

export async function checkConnection(
  client: S3Client,
  bucket: string,
): Promise<void> {
  await client.send(new HeadBucketCommand({ Bucket: bucket }));
}

/**
 * Streaming multipart upload that accumulates chunks into 8MB parts.
 */
export class MultipartUpload {
  private client: S3Client;
  private bucket: string;
  private key: string;
  private uploadId: string = "";
  private parts: { ETag: string; PartNumber: number }[] = [];
  private partBuffer: Uint8Array[] = [];
  private partBufferSize = 0;
  private partNumber = 1;
  private totalUploaded = 0;
  private readonly minPartSize = 8 * 1024 * 1024; // 8 MB

  constructor(client: S3Client, bucket: string, key: string) {
    this.client = client;
    this.bucket = bucket;
    this.key = key;
  }

  async start(): Promise<void> {
    const resp = await this.client.send(
      new CreateMultipartUploadCommand({
        Bucket: this.bucket,
        Key: this.key,
        ContentType: "video/mp4",
      }),
    );
    this.uploadId = resp.UploadId!;
  }

  async addChunk(data: Uint8Array): Promise<void> {
    this.partBuffer.push(data);
    this.partBufferSize += data.length;

    if (this.partBufferSize >= this.minPartSize) {
      await this.flushPart();
    }
  }

  private async flushPart(): Promise<void> {
    if (this.partBufferSize === 0) return;

    // Concatenate buffered chunks into one part
    const part = new Uint8Array(this.partBufferSize);
    let offset = 0;
    for (const chunk of this.partBuffer) {
      part.set(chunk, offset);
      offset += chunk.length;
    }
    this.partBuffer = [];
    this.partBufferSize = 0;

    const resp = await this.client.send(
      new UploadPartCommand({
        Bucket: this.bucket,
        Key: this.key,
        UploadId: this.uploadId,
        PartNumber: this.partNumber,
        Body: part,
      }),
    );

    this.parts.push({
      ETag: resp.ETag!,
      PartNumber: this.partNumber,
    });
    this.partNumber++;
    this.totalUploaded += part.length;
  }

  async finish(): Promise<number> {
    // Flush remaining data as the final part
    await this.flushPart();

    await this.client.send(
      new CompleteMultipartUploadCommand({
        Bucket: this.bucket,
        Key: this.key,
        UploadId: this.uploadId,
        MultipartUpload: { Parts: this.parts },
      }),
    );

    return this.totalUploaded;
  }

  getTotalUploaded(): number {
    return this.totalUploaded;
  }
}

/**
 * Copy an object to a new key using multipart copy (handles >5GB).
 */
export async function multipartCopy(
  client: S3Client,
  bucket: string,
  sourceKey: string,
  destKey: string,
  totalSize: number,
): Promise<void> {
  const partSize = 100 * 1024 * 1024; // 100 MB parts for copy
  const numParts = Math.ceil(totalSize / partSize);

  if (numParts <= 1) {
    // Single-part copy (< 5GB)
    const { UploadId } = await client.send(
      new CreateMultipartUploadCommand({
        Bucket: bucket,
        Key: destKey,
        ContentType: "video/mp4",
      }),
    );

    const { CopyPartResult } = await client.send(
      new UploadPartCopyCommand({
        Bucket: bucket,
        Key: destKey,
        UploadId: UploadId!,
        PartNumber: 1,
        CopySource: `${bucket}/${sourceKey}`,
      }),
    );

    await client.send(
      new CompleteMultipartUploadCommand({
        Bucket: bucket,
        Key: destKey,
        UploadId: UploadId!,
        MultipartUpload: {
          Parts: [{ ETag: CopyPartResult!.ETag!, PartNumber: 1 }],
        },
      }),
    );
    return;
  }

  // Multi-part copy for large objects
  const { UploadId } = await client.send(
    new CreateMultipartUploadCommand({
      Bucket: bucket,
      Key: destKey,
      ContentType: "video/mp4",
    }),
  );

  const parts: { ETag: string; PartNumber: number }[] = [];
  for (let i = 0; i < numParts; i++) {
    const start = i * partSize;
    const end = Math.min(start + partSize, totalSize) - 1;
    const { CopyPartResult } = await client.send(
      new UploadPartCopyCommand({
        Bucket: bucket,
        Key: destKey,
        UploadId: UploadId!,
        PartNumber: i + 1,
        CopySource: `${bucket}/${sourceKey}`,
        CopySourceRange: `bytes=${start}-${end}`,
      }),
    );
    parts.push({ ETag: CopyPartResult!.ETag!, PartNumber: i + 1 });
  }

  await client.send(
    new CompleteMultipartUploadCommand({
      Bucket: bucket,
      Key: destKey,
      UploadId: UploadId!,
      MultipartUpload: { Parts: parts },
    }),
  );
}

/**
 * Upload a small object (init segment, metadata JSON).
 */
export async function putObject(
  client: S3Client,
  bucket: string,
  key: string,
  data: Uint8Array | string,
  contentType: string,
): Promise<void> {
  const body = typeof data === "string" ? new TextEncoder().encode(data) : data;
  await client.send(
    new PutObjectCommand({
      Bucket: bucket,
      Key: key,
      Body: body,
      ContentType: contentType,
    }),
  );
}

/**
 * Delete an object.
 */
export async function deleteObject(
  client: S3Client,
  bucket: string,
  key: string,
): Promise<void> {
  await client.send(
    new DeleteObjectCommand({
      Bucket: bucket,
      Key: key,
    }),
  );
}
