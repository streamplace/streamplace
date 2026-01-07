// Web translation loader - loads translations via fetch for code splitting
// Metro will use this file for web builds

export async function loadTranslationData(
  locale: string,
  namespace: string,
): Promise<any> {
  const response = await fetch(`/locales/${locale}/${namespace}.json`);
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }
  const translations = await response.json();

  if (!translations || Object.keys(translations).length === 0) {
    throw new Error("No translations found");
  }

  return translations;
}
