// localStorage wrapper matching the @streamplace/components `AQStorage`
// interface (`getItem` / `setItem` / `removeItem`, all Promise-returning).
// The app's slices import `storage` from `@streamplace/components`; the web
// port imports it from here. The shape is what matters, not the mechanism.
//
// SSR guard: web is a Vite SPA, so `localStorage` is always present at
// runtime. The guard exists so module evaluation never throws under
// build-time tools that try to evaluate the module in a non-browser context.

const localStorage_ = (): Storage | null => {
  if (typeof localStorage === "undefined") return null;
  return localStorage;
};

export const storage = {
  async getItem(key: string): Promise<string | null> {
    return localStorage_()?.getItem(key) ?? null;
  },
  async setItem(key: string, value: string): Promise<void> {
    localStorage_()?.setItem(key, value);
  },
  async removeItem(key: string): Promise<void> {
    localStorage_()?.removeItem(key);
  },
};
