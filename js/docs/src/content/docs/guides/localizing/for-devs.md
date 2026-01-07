---
title: Localization for Developers
description: Things to consider when writing content that will be localized.
sidebar:
  order: 20
---

Hello! Here are some things to keep in mind when writing content that includes
copy that will be localized.

This includes (but is not limited to):

- Button prompts
- Menu items
- Error messages
- Notifications
- Any other text that users will see

This is **NOT**:

- User submitted content (e.g. chat messages, stream titles, etc)

## Quick start

1. Most of the time, you'll use the useTranslation hook from `react-i18next` to
   wrap any text that will be localized:

```ts
import { useTranslation } from 'react-i18next';

function MyComponent() {
  // Use default namespace (common)
  const { t } = useTranslation();

  return (
    <div>
      {/* Simple translation from common namespace */}
      <button>{t('save')}</button>
      <button>{t('cancel')}</button>

      {/* With variables */}
      <p>{t('welcome-user', { username: 'Alice' })}</p>

      {/* With count (triggers pluralization) */}
      <span>{t('notification-count', { count: 5 })}</span>
    </div>
  );
}

function SettingsComponent() {
  // Specify a namespace for settings-specific translations
  const { t } = useTranslation('settings');

  return (
    <div>
      <h1>{t('settings-title')}</h1>
      <p>{t('language-selection-description')}</p>
    </div>
  );
}
```

2. For complex React components, use the `Trans` function from `react-i18next`
   to wrap any text that will be localized. As long you have no React/HTML nodes
   integrated into a cohesive sentence (text formatting like strong, em, link
   components, maybe others), you won't need it. Most of the times you will be
   using the above `t` function. You should probably import it as `T` to save
   some typing:

```tsx
import { Trans as T } from "react-i18next";
import { Text, Linking } from "react-native";

function HelpText() {
  const handleEmailPress = () => {
    Linking.openURL("mailto:support@stream.place");
  };

  const handleDocsPress = () => {
    Linking.openURL("/docs"); // Or use your navigation method
  };

  return (
    <T
      i18nKey="help-message"
      components={{
        supportLink: (
          <Text
            style={{ color: "#007AFF", textDecorationLine: "underline" }}
            onPress={handleEmailPress}
          />
        ),
        docsLink: (
          <Text
            style={{ color: "#007AFF", textDecorationLine: "underline" }}
            onPress={handleDocsPress}
          />
        ),
        bold: <Text style={{ fontWeight: "bold" }} />,
      }}
      values={{
        username: "Alice",
        supportEmail: "support@stream.place",
      }}
    >
      Hi <bold>{{ username }}</bold>! Need help? Contact{" "}
      <supportLink>{{ supportEmail }}</supportLink>
      or check our <docsLink>documentation</docsLink>.
    </T>
  );
}
```

## Workflow

1. Add translation keys in your code as shown above.

```ts
// ❌ Don't hardcode text
<Text>Settings</Text>
<Text>You have 5 new messages</Text>

// ✅ Use translation keys with appropriate namespace
// Common UI elements use default namespace
<Button>{t('save')}</Button>
<Button>{t('cancel')}</Button>

// Settings-specific use settings namespace
const { t } = useTranslation('settings');
<Text>{t('settings-title')}</Text>
```

2. Add translations to the appropriate .ftl files in `js/components/locales/`:

**For common UI elements** (buttons, labels, etc.), use `common.ftl`:

```fluent
# Edit: js/components/locales/en-US/common.ftl
save = Save
cancel = Cancel
loading = Loading...
error = Error
```

**For feature-specific translations**, use the appropriate namespace file:

```fluent
# Edit: js/components/locales/en-US/settings.ftl
settings-title = Settings
language-selection = Language
language-selection-description = Choose your preferred language
```

3. Compile the translations to JSON:

```bash
cd js/components
pnpm i18n:compile
```

This reads the `.ftl` files and outputs compiled JSON to
`js/components/public/locales/{locale}/{namespace}.json` (e.g., `common.json`,
`settings.json`).

4. For web: the compiled files in `public/locales/` are served as static assets
   and loaded on demand.

For native: the compiled files are bundled with the app via static `require()`
calls in the components package.

## Project structure

The i18n system is centralized in `@streamplace/components`:

```
js/components/
├── locales/                   # Source .ftl files (organized by namespace)
│   ├── en-US/
│   │   ├── common.ftl         # Common UI elements (buttons, labels, etc.)
│   │   └── settings.ftl       # Settings-specific translations
│   ├── pt-BR/
│   ├── es-ES/
│   ├── zh-Hant/
│   └── fr-FR/
├── src/i18n/
│   ├── manifest.json          # Supported locales and metadata
│   ├── i18next-config.ts      # Bootstrap configuration with namespace setup
│   ├── provider.tsx           # React provider components
│   └── index.ts               # Public exports
├── public/locales/            # Compiled JSON output (by locale and namespace)
│   ├── en-US/
│   │   ├── common.json
│   │   └── settings.json
│   └── ...
└── scripts/
    ├── compile-translations.js  # Uses @fluent/syntax to parse .ftl files
    └── extract-i18n.js
```

The app imports i18n from `@streamplace/components`:

```ts
import { i18next, useTranslation } from "@streamplace/components";
```

## Namespaces

Translations are organized into **namespaces** to keep related translations
together and improve code organization:

- **`common`** (default): General UI elements used across the app

  - Buttons: save, cancel, delete, edit, etc.
  - Status messages: loading, error, success, warning
  - Common actions: yes, no, continue, back, next

- **`settings`**: Settings page translations
  - App version and update messages
  - Language selection
  - Custom node configuration
  - Debug recording settings

When adding new feature areas, create a new namespace `.ftl` file:

1. Create `locales/{locale}/feature.ftl` for each locale
2. Add the namespace to `I18NEXT_CONFIG.ns` array in `i18next-config.ts`
3. Add static `require()` entries for React Native in the translation map
4. Use `useTranslation('feature')` in your components

## Available scripts

In `js/components`:

- `pnpm i18n:compile` - Compile .ftl files to JSON and copy to app
- `pnpm i18n:watch` - Watch .ftl files, recompile and copy on changes
  (recommended for development)
- `pnpm i18n:extract` - Extract translation keys from source code and add them
  to .ftl files

### Development workflow

When actively working on translations:

1. In one terminal, run the app dev server:

   ```bash
   cd js/app
   pnpm start  # or pnpm web, pnpm ios, etc.
   ```

2. In another terminal, run the translation watcher:

   ```bash
   cd js/components
   pnpm i18n:watch
   ```

3. Edit `.ftl` files in `js/components/locales/`

4. Save - the watcher will automatically:
   - Compile your changes to JSON
   - Copy the compiled files to `js/app/public/locales/`
   - Your dev server will pick up the changes and hot reload

This provides a fast feedback loop for translation work!

## Keep in mind...

### Keys are important

Choose meaningful keys that describe the content, not the presentation. They
should be descriptive and scoped.

```fluent
user-welcome-message = Welcome back, { $username }!
settings-account-title = Account Settings
error-network-connection = Connection failed
button-save-changes = Save Changes
form-validation-email-invalid = Please enter a valid email address
```

### Platform differences

The system handles both web and React Native:

- **Web**: loads translations via HTTP from `/locales/{locale}/messages.json`
- **React Native**: bundles translations via static `require()` calls

The bootstrap code in `@streamplace/components/i18n` automatically detects the
platform and uses the appropriate loading strategy.

### Adding new locales

1. Add the locale to `js/components/src/i18n/manifest.json`:

```json
{
  "supportedLocales": [
    "en-US",
    "pt-BR",
    "es-ES",
    "zh-Hant",
    "fr-FR",
    "new-LOCALE"
  ],
  "languages": {
    "new-LOCALE": {
      "code": "new-LOCALE",
      "name": "Language Name",
      "nativeName": "Native Name",
      "flag": "🏁"
    }
  }
}
```

2. Create the locale directory and copy .ftl files from another locale:

```bash
cd js/components/locales
mkdir new-LOCALE
cp en-US/*.ftl new-LOCALE/
```

3. Translate the .ftl files in `js/components/locales/new-LOCALE/`

4. Add static `require()` entries for all namespaces in
   `js/components/src/i18n/i18next-config.ts`:

```ts
const translationMap: Record<string, any> = {
  // ... existing entries
  "new-LOCALE/common": require("../../public/locales/new-LOCALE/common.json"),
  "new-LOCALE/settings": require("../../public/locales/new-LOCALE/settings.json"),
  // Add any other namespaces you've created
};
```

5. Run `pnpm i18n:compile` to generate the JSON files

You can also
[view the official Fluent docs](https://github.com/projectfluent/fluent/wiki/Good-Practices-for-Developers)
and
[the official syntax guide](https://projectfluent.org/fluent/guide/index.html)
to learn how to write better Fluent messages.
