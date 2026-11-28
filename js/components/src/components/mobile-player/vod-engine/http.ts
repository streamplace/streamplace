// Segment bytes over HTTP: exactly the byte range the HLS media playlist names.
export async function fetchRange(
  url: string,
  offset: number,
  size: number,
  signal: AbortSignal,
): Promise<Uint8Array<ArrayBuffer>> {
  const res = await fetch(url, {
    headers: { Range: `bytes=${offset}-${offset + size - 1}` },
    signal,
  });
  if (res.status !== 206) throw new Error(`segment fetch: HTTP ${res.status}`);
  return new Uint8Array(await res.arrayBuffer());
}
