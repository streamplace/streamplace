// base-ui's primitives accept a state-aware className function in addition
// to a plain string. The function form is `(state) => string | undefined`
// where `state` is a base-ui state object. We accept any function shape in
// the type system and try invoking it with no args at runtime — fine for
// thunks that read state from closure; silently returns "" if the function
// actually requires a state argument (which our wrappers never pass through).
type ClassValue =
  | string
  | boolean
  | undefined
  | null
  | ((...args: any[]) => string | undefined);

export function cn(...inputs: ClassValue[]): string {
  return inputs
    .map((input) => {
      if (typeof input === "function") {
        try {
          return input();
        } catch {
          return "";
        }
      }
      return input;
    })
    .filter(Boolean)
    .join(" ");
}
