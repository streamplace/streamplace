const VIP_BADGE_TYPE = "place.stream.badge.defs#vip";

export function formatBadgeLabel({
  badgeType,
  badgeName,
  fallbackLabel,
  vipLabel,
}: {
  badgeType: string;
  badgeName?: string | null;
  fallbackLabel: string;
  vipLabel: string;
}): string {
  const name = badgeName?.trim();
  if (badgeType === VIP_BADGE_TYPE && name) {
    return `(${vipLabel}) ${name}`;
  }
  return fallbackLabel;
}

export function formatBadgeIssuer(issuer: string, handle?: string): string {
  const normalizedHandle = handle?.trim().replace(/^@/, "");
  if (normalizedHandle) return `@${normalizedHandle}`;
  if (issuer.length <= 16) return issuer;
  return `${issuer.slice(0, 12)}…`;
}
