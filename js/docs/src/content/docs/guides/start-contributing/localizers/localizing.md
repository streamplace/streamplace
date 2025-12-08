---
title: Localization Guide for Translators
description: How to contribute translations to Streamplace
sidebar:
  order: 10
---

This guide will help you contribute translations to make Streamplace more
accessible to users around the world.

## What we translate

Streamplace uses **Fluent** (developed by Mozilla) for internationalization.
You'll be translating user-facing text such as:

- **Button labels**: Save, Cancel, Delete, Settings, etc.
- **Menu items**: Navigation labels, dropdown options
- **Messages**: Error messages, notifications, success messages
- **Form labels**: Input field descriptions, validation messages
- **Help text**: Tooltips, instructions, descriptions

**What we DON'T translate:**

- User-generated content (chat messages, stream titles, usernames)
- Code comments or technical documentation
- API endpoints or internal system messages

## Getting started

:::caution[Under Development] Translations are under rapid development. Most of
this is final, but some may change.

A localization service is in the works. For discussion, please
[visit the Discord](https://discord.stream.place). :::

### 1. File organization

All translation files are located in `js/components/locales/` and organized by
language:

```
js/components/locales/
├── en-US/           # English (US) - source language
│   ├── common.ftl   # General UI elements
│   └── settings.ftl # Settings page
├── pt-BR/           # Portuguese (Brazil)
├── es-ES/           # Spanish (Spain)
├── zh-Hant/         # Traditional Chinese
└── fr-FR/           # French (France)
```

### 2. Translation file format

We use **Fluent (.ftl)** files. Here's what they look like:

```fluent
# Comments start with # and help explain context
save = Save
cancel = Cancel

# Variables are wrapped in curly braces
welcome-message = Welcome back, { $username }!

# Pluralization handles different forms based on count
notification-count = { $count ->
    [0] No new notifications
    [1] 1 new notification
   *[other] { $count } new notifications
}
```

### 3. Translation namespaces

Translations are organized into **namespaces** (categories). For example:

- **`common.ftl`**: General UI elements used throughout the app
- **`settings.ftl`**: Settings page translations, for keys under
  [`/settings`](/settings)

## Translation workflow

### Step 1: Choose your language

Check if your language already exists in `js/components/locales/`. If not, you
can request a new language by opening an issue or contacting the maintainers.

### Step 2: Work with .ftl files

Each `.ftl` file contains translation keys and their values:

```fluent
# Simple translation
settings-title = Settings

# Translation with a variable
language-changed = Language changed to { $language }

# Translation with pluralization
files-selected = { $count ->
    [0] No files selected
    [1] 1 file selected
   *[other] { $count } files selected
}
```

### Step 3: Understanding context

#### Translation keys

Keys describe what the text is for, not what it says:

```fluent
# ✅ Good - describes purpose
button-save-changes = Save Changes
error-network-connection = Unable to connect to server

# ❌ Avoid - describes appearance
blue-button-text = Save Changes
top-error-message = Connection Error
```

#### Variables

Variables are marked with `$` and should not be translated:

```fluent
# The $username will be replaced with actual usernames
user-profile-title = { $username }'s Profile

# Multiple variables
stream-info = { $viewerCount } viewers watching { $streamerName }
```

#### Pluralization

Use Fluent's pluralization features for different languages:

```fluent
# English pluralization
item-count = { $count ->
    [0] No items
    [1] 1 item
   *[other] { $count } items
}

# Russian has different plural rules
item-count = { $count ->
    [one] { $count } элемент
    [few] { $count } элемента
   *[other] { $count } элементов
}
```

### Step 4: Testing translations

While technical setup is handled by developers, you can:

1. **Review in context**: Ask developers to show you how translations appear in
   the app
2. **Check length**: Some translations may need to be shorter/longer to fit the
   UI
3. **Verify variables**: Make sure variables like `{ $username }` are properly
   placed

## Best practices

### 1. Maintain consistency

- Use the same terms for the same concepts throughout
- Keep a glossary of important terms and their translations
- Follow your language's style guides and conventions

### 2. Consider cultural context

- Adapt messages to feel natural in your language
- Use appropriate levels of formality
- Consider regional variations when relevant

### 3. Handle technical terms

- Keep technical terms in English when commonly understood (e.g., "email",
  "username")
- Translate when good native equivalents exist
- Be consistent with your choices

### 4. Work with variables and formatting

Variables should be positioned naturally in your language:

```fluent
# English: subject-verb-object
welcome-back = Welcome back, { $username }!

# Japanese: might be structured differently
welcome-back = { $username }さん、おかえりなさい！
```

### 5. Length considerations

Some UI elements may have space constraints. Feel free to load up the app and
check if you're comfortable.

```fluent
# Button text should be concise
save = Save
cancel = Cancel

# Error messages can be longer
password-requirements = Password must be at least 8 characters long and contain both letters and numbers
```

## Common Fluent syntax

### Basic message

```fluent
hello = Hello
goodbye = Goodbye
```

### With variables

```fluent
greeting = Hello, { $name }!
```

### With pluralization

```fluent
inbox = { $count ->
    [0] Empty inbox
    [1] 1 message
   *[other] { $count } messages
}
```

### With attributes (for accessibility)

```fluent
close-button = Close
    .aria-label = Close dialog
```

### Referencing other messages

```fluent
brand-name = Streamplace
welcome-message = Welcome to { brand-name }!
```

## Getting help

- **Fluent documentation**: [projectfluent.org](https://projectfluent.org/)
- **Fluent syntax guide**:
  [Fluent guide](https://projectfluent.org/fluent/guide/)
- **Questions**: Open an issue on GitHub or contact the maintainers
- **Context needed**: If you're unsure about a translation's context, feel free
  to ask on the [Discord](https://discord.stream.place), and we'll be happy to
  help out.

## Quality checklist

Before submitting translations:

- [ ] All translation keys have values (no empty translations)
- [ ] Variables like `{ $username }` are preserved and positioned correctly
- [ ] Pluralization rules match your language's grammar
- [ ] Text sounds natural and uses consistent terminology
- [ ] Special characters and punctuation are appropriate for your language
- [ ] Translations fit the intended context (button vs. paragraph text)

## Contributing

1. **Fork the repository** or work directly with maintainers
2. **Edit .ftl files** in your language directory
3. **Test your changes** (coordinate with developers)
4. **Submit your translations** via pull request or by sharing files

Thank you for helping make Streamplace accessible to speakers of your language!
Your contributions help create a more inclusive platform for creators worldwide.
