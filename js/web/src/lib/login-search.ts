// Satisfies TanStack Router's required search params shape for /login links.
export const EMPTY_LOGIN_SEARCH = {
  code: undefined,
  state: undefined,
  iss: undefined,
  error: undefined,
  errorDescription: undefined,
} as const;
