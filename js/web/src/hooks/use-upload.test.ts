import { describe, expect, it } from "vitest";
import { humanBytes, validateVideoFile } from "../hooks/use-upload";

describe("humanBytes", () => {
  it("formats bytes", () => {
    expect(humanBytes(0)).toBe("0 B");
    expect(humanBytes(512)).toBe("512 B");
    expect(humanBytes(1023)).toBe("1023 B");
  });

  it("formats kilobytes", () => {
    expect(humanBytes(1024)).toBe("1.0 KB");
    expect(humanBytes(1536)).toBe("1.5 KB");
    expect(humanBytes(1048575)).toBe("1024.0 KB");
  });

  it("formats megabytes", () => {
    expect(humanBytes(1048576)).toBe("1.0 MB");
    expect(humanBytes(5242880)).toBe("5.0 MB");
  });

  it("formats gigabytes", () => {
    expect(humanBytes(1073741824)).toBe("1.00 GB");
    expect(humanBytes(5368709120)).toBe("5.00 GB");
  });
});

describe("validateVideoFile", () => {
  async function makeFile(bytes: number[], type = "video/mp4"): Promise<File> {
    const buf = new Uint8Array(bytes);
    return new File([buf], "test", { type });
  }

  it("accepts MP4 (ftyp box at offset 4)", async () => {
    // 4 zero bytes + "ftyp" + remaining
    const file = await makeFile([
      0, 0, 0, 0, 0x66, 0x74, 0x79, 0x70, 0, 0, 0, 0,
    ]);
    expect(await validateVideoFile(file)).toBeNull();
  });

  it("accepts WebM/MKV (EBML magic)", async () => {
    const file = await makeFile([
      0x1a, 0x45, 0xdf, 0xa3, 0, 0, 0, 0, 0, 0, 0, 0,
    ]);
    expect(await validateVideoFile(file)).toBeNull();
  });

  it("accepts AVI (RIFF + AVI)", async () => {
    const file = await makeFile([
      0x52, 0x49, 0x46, 0x46, 0, 0, 0, 0, 0x41, 0x56, 0x49, 0x20,
    ]);
    expect(await validateVideoFile(file)).toBeNull();
  });

  it("accepts FLV", async () => {
    const file = await makeFile([0x46, 0x4c, 0x56, 0, 0, 0, 0, 0, 0, 0, 0, 0]);
    expect(await validateVideoFile(file)).toBeNull();
  });

  it("accepts MPEG-TS (sync byte 0x47)", async () => {
    const file = await makeFile([0x47, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]);
    expect(await validateVideoFile(file)).toBeNull();
  });

  it("rejects non-video files", async () => {
    const file = await makeFile([
      0x89, 0x50, 0x4e, 0x47, 0, 0, 0, 0, 0, 0, 0, 0,
    ]);
    const result = await validateVideoFile(file);
    expect(result).not.toBeNull();
    expect(result).toContain("supported video format");
  });

  it("rejects empty data", async () => {
    const file = await makeFile([0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]);
    const result = await validateVideoFile(file);
    expect(result).not.toBeNull();
  });
});
