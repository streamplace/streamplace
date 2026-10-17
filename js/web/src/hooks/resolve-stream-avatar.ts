export function resolveStreamAvatar({
  detailedAvatar,
  profileAvatar,
  authorAvatar,
}: {
  detailedAvatar?: string | null;
  profileAvatar?: string | null;
  authorAvatar?: string | null;
}): string | undefined {
  return detailedAvatar ?? profileAvatar ?? authorAvatar ?? undefined;
}
