// The /login route doubles as the OAuth callback target.
export function getCallbackUrl(): string {
  return "/login";
}
