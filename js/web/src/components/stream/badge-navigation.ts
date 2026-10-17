export type BadgeNavigationDirection = -1 | 1;

export function getAdjacentBadgeIndex(
  currentIndex: number,
  badgeCount: number,
  direction: BadgeNavigationDirection,
): number | null {
  if (badgeCount <= 0) return null;
  return (currentIndex + direction + badgeCount) % badgeCount;
}
