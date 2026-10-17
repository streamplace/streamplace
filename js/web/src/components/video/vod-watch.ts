export function getAuthorHandle(
  author: { handle?: string; displayName?: string } | null | undefined,
  fallback: string,
): string {
  return author?.handle?.trim() || fallback;
}

export function buildVodShareLinks(origin: string, user: string, tid: string) {
  const base = new URL(origin).origin;
  const route = `${encodeURIComponent(user)}/video/${encodeURIComponent(tid)}`;
  const pageUrl = `${base}/${route}`;
  const embedUrl = `${base}/embed/${route}`;

  return {
    pageUrl,
    embedUrl,
    embedCode: `<iframe src="${embedUrl}" width="640" height="360" frameborder="0" allowfullscreen></iframe>`,
  };
}

export function getOptimisticLikeState({
  liked,
  count,
}: {
  liked: boolean;
  count: number;
}) {
  return {
    liked: !liked,
    count: liked ? Math.max(0, count - 1) : count + 1,
  };
}

export function shouldCollapseDescription(
  description: string,
  previewLength = 180,
): boolean {
  return description.trim().length > previewLength;
}
