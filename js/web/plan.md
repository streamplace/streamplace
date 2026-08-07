# Streamplace Web App: Migration Plan

## What We're Doing

The existing React Native app (`js/app`) is the source of truth. The new web app (`js/web`) is a fresh Vite + TanStack Router SPA built with true DOM, Tailwind, and Base UI. It's already a solid foundation but has explicit Phase 3/4 stubs for core streaming features. This plan prioritizes reaching full feature parity with the RN app, then layering on web-native advantages.

## Where We Are Today

### Web App: What Works (Phase 1-2)

- Home feed with live user grid
- Single live stream viewer (HLS + WebRTC)
- Single VOD viewer
- VOD browse
- VOD upload (tus-js-client, metadata, thumbnails, progress)
- OAuth login/registration
- Chat (send/receive, mentions, emoji)
- Settings: account, streaming (keys with create/delete), recommendations, webhooks, multistream, privacy, backup, branding, badges, languages, advanced, about
- Player with stats overlay, keyboard shortcuts, quality selector
- i18n (7 locales via `@streamplace/i18n`)

### Web App: Explicitly Stubbed (Phase 3/4)

From `blueskySlice.ts` header comments:

- `createLivestreamRecord` - throws
- `updateLivestreamRecord` - throws
- `golivePost` - throws
- `createBlockRecord` - throws (low priority, not in RN either)

### Web App: Missing Entirely

- Go-live flow (no route, no UI, no broadcaster backend)
- Following feed (sidebar "Following" is disabled)
- Notifications surface (no route, no header bell)
- Multi-stream viewer (grid of multiple live streams; `dashboard/multistream/` is for RTMP target management, not viewing)
- Popout windows (livestream, multistream, stream monitor, info widget)
- Download screen
- Support screen
- "What is Streamplace?" about screen (web has `/settings/about` but that's settings-scoped, not a public landing page)

### RN App: Feature Inventory

All screens in `js/app/src/screens/`:

| Screen                      | Purpose                                    |
| --------------------------- | ------------------------------------------ |
| `home.tsx`                  | Live stream grid with pull-to-refresh      |
| `mobile-stream.tsx`         | Live stream viewer (full-screen on mobile) |
| `mobile-go-live.tsx`        | Go-live with ingest player                 |
| `launch-go-live.tsx`        | "Ready to go live?" prompt                 |
| `live-dashboard.tsx`        | BentoGrid broadcaster dashboard            |
| `video-list.tsx`            | VOD browse                                 |
| `video.tsx`                 | Single VOD playback                        |
| `vod.tsx`                   | VOD screen                                 |
| `upload.tsx`                | VOD upload (1484 lines, tus-js-client)     |
| `chat-popout.tsx`           | Popout chat window                         |
| `multi.tsx`                 | Multi-stream viewer                        |
| `about.tsx`                 | "What is Streamplace?"                     |
| `download.tsx`              | Download app                               |
| `embed.tsx`                 | Stream embed                               |
| `vod-embed.tsx`             | VOD embed                                  |
| `danmu-obs.tsx`             | Danmu OBS overlay                          |
| `info-widget-embed.tsx`     | Info widget embed                          |
| `popout-livestream.tsx`     | Popout livestream window                   |
| `popout-multistream.tsx`    | Popout multistream window                  |
| `popout-stream-monitor.tsx` | Stream monitor popout                      |
| `popout-info-widget.tsx`    | Info widget popout                         |
| `support.tsx`               | Support page                               |
| `app-return.tsx`            | OAuth return handling                      |

### Screen-by-Screen Comparison

| RN Screen                   | Web Equivalent                 | Status     | Notes                                                                                    |
| --------------------------- | ------------------------------ | ---------- | ---------------------------------------------------------------------------------------- |
| `home.tsx`                  | `$user/index.tsx`, `index.tsx` | ✅ Parity  | Web has infinite scroll instead of pull-to-refresh. Both show live grid + viewer counts. |
| `mobile-stream.tsx`         | `$user/index.tsx`              | ⚠️ Partial | Web is desktop-focused. No dedicated mobile-optimized live view.                         |
| `mobile-go-live.tsx`        | —                              | ❌ Missing | Mobile camera ingest, no web equivalent.                                                 |
| `launch-go-live.tsx`        | —                              | ❌ Missing | "Ready to go live?" prompt, no web equivalent.                                           |
| `live-dashboard.tsx`        | `dashboard/stream/index.tsx`   | ✅ Parity  | Web has additional metadata/moderation sections.                                         |
| `video-list.tsx`            | `videos.tsx`                   | ✅ Parity  | Both use same API, infinite scroll, responsive grid.                                     |
| `video.tsx`                 | `$user/video/$tid.tsx`         | ✅ Parity  | Single VOD playback.                                                                     |
| `vod.tsx`                   | `$user/video/$tid.tsx`         | ✅ Merged  | RN has separate vod.tsx and video.tsx; web combines into one route.                      |
| `upload.tsx`                | `dashboard/upload.tsx`         | ✅ Done    | tus upload, metadata form, thumbnail, progress. In creator dashboard.                    |
| `chat-popout.tsx`           | `chat-popout.$user.tsx`        | ✅ Done    | Standalone chat window.                                                                  |
| `multi.tsx`                 | —                              | ❌ Missing | Grid of multiple simultaneous live streams. Not the same as dashboard/multistream.       |
| `about.tsx`                 | `settings/about.tsx`           | ⚠️ Partial | Web version is settings-scoped. RN has a public "What is Streamplace?" page.             |
| `download.tsx`              | —                              | ❌ Missing | App download/promo page with store links.                                                |
| `embed.tsx`                 | `embed/$user/index.tsx`        | ✅ Done    | Minimal-chrome stream embed.                                                             |
| `vod-embed.tsx`             | `embed/$user/video/$tid.tsx`   | ✅ Done    | VOD embed.                                                                               |
| `danmu-obs.tsx`             | `embed/danmu-obs/$user.tsx`    | ✅ Done    | Danmu OBS overlay, full feature parity.                                                  |
| `info-widget-embed.tsx`     | `embed/info-widget/$user.tsx`  | ✅ Done    | Info widget embed.                                                                       |
| `popout-livestream.tsx`     | —                              | ❌ Missing | Popout livestream player window.                                                         |
| `popout-multistream.tsx`    | —                              | ❌ Missing | Popout multistream window.                                                               |
| `popout-stream-monitor.tsx` | —                              | ❌ Missing | Popout stream monitor.                                                                   |
| `popout-info-widget.tsx`    | —                              | ❌ Missing | Covered by `/embed/info-widget/$user` but no dedicated popout route.                     |
| `support.tsx`               | —                              | ❌ Missing | Redirects to Google Form.                                                                |
| `app-return.tsx`            | Handled by `login.tsx`         | ✅ Done    | OAuth callback handled differently on web.                                               |

### Web Features Beyond RN Parity

| Feature                            | Status  | Notes                                                        |
| ---------------------------------- | ------- | ------------------------------------------------------------ |
| Search (header bar + popover)      | ✅ Done | RN has no equivalent search UI.                              |
| Danmu overlay                      | ✅ Done | Bullet comments with lane assignment, mobile parity.         |
| Theatre mode                       | ✅ Done | Sidebar + header hidden, `T` shortcut. RN has no equivalent. |
| Picture-in-Picture                 | ✅ Done | Web-native PiP API. RN has no equivalent.                    |
| Dashboard webhooks/branding/badges | ✅ Done | Advanced settings panels not in RN.                          |
| Keyboard shortcuts                 | ✅ Done | Space, m, f, T shortcuts. RN has no equivalent.              |

### RN App: Navigation Tree

```
RootStack
├── MainTabs
│   ├── HomeTab → HomeNavigator
│   │   ├── HomeMain (home.tsx)
│   │   ├── About
│   │   ├── Download
│   │   ├── LiveDashboard
│   │   ├── Login
│   │   ├── Multi
│   │   ├── Support
│   │   └── Upload (web only)
│   ├── VideosTab → VideosNavigator
│   │   ├── VideoList
│   │   └── UserVideoList
│   ├── GoLiveTab → LaunchGoLive
│   └── SettingsTab → SettingsNavigator
│       ├── MainSettings
│       ├── AboutCategory
│       ├── AccountCategory
│       ├── StreamingCategory
│       ├── WebhooksSettings
│       ├── BackupSettings
│       ├── RecommendationsSettings
│       ├── PrivacyCategory
│       ├── DanmuCategory
│       ├── AdvancedCategory
│       ├── MultistreamCategory
│       ├── LanguagesCategory
│       ├── KeyManagement
│       ├── BadgeSelection
│       ├── BadgeIssuer
│       └── BrandingAdmin
├── Stream (MobileStream)
├── MobileGoLive
├── Video
├── Vod
├── PopoutChat
├── Embed / VodEmbed / InfoWidgetEmbed / DanmuOBS
├── PopoutStreamMonitor / PopoutInfoWidget / PopoutMultistream / PopoutLivestream
└── AppReturn
```

### Shared Packages

| Package                         | Web Safe?    | Used by Web? | Notes                                                                                  |
| ------------------------------- | ------------ | ------------ | -------------------------------------------------------------------------------------- |
| `@streamplace/core`             | Mostly yes   | Yes          | LivestreamStore, handleWebSocketMessages, reduceChat, segmentize, Facet                |
| `@streamplace/i18n`             | Yes          | Yes          | manifest, createI18nextConfig, locale resources                                        |
| `@streamplace/components`       | No (RN deps) | No           | Contains Player, settings panels, hooks, but depends on react-native, expo-video, etc. |
| `streamplace`                   | Yes          | Yes          | PlaceStream\* types, agent methods                                                     |
| `js/playback-router`            | Likely yes   | No           | Not imported by web, worker-specific                                                   |
| `js/config-react-native-webrtc` | No           | No           | RN-specific                                                                            |

### RN Platform Dependencies in Active Use

| Dependency                                                | Used In (RN)                  | Web Equivalent                                                    |
| --------------------------------------------------------- | ----------------------------- | ----------------------------------------------------------------- |
| `expo-video`                                              | Player components             | Native `<video>` + hls.js (already done)                          |
| `react-native-webrtc`                                     | WebRTC publishing/subscribing | Native `RTCPeerConnection` (subscribing done, publishing missing) |
| `expo-notifications` + `@react-native-firebase/messaging` | Push notifications            | Web Push API / Notification API                                   |
| `expo-keep-awake`                                         | Go-live, live dashboard       | `navigator.wakeLock.request("screen")`                            |
| `expo-screen-orientation`                                 | Full-screen video             | Screen Orientation API (limited)                                  |
| `expo-sqlite`                                             | Local caching                 | IndexedDB                                                         |
| `expo-file-system`                                        | File downloads, uploads       | Fetch API / File System Access API                                |
| `expo-image`                                              | Image loading                 | Native `<img>` / `<picture>`                                      |
| `expo-document-picker`                                    | File selection                | `<input type="file">`                                             |
| `react-native-markdown-display`                           | Chat messages                 | TipTap (already done) or markdown-it                              |
| `@zxing/browser`                                          | QR code scanning              | Native barcode detection or html5-qrcode                          |
| `react-native-reanimated`                                 | Animations                    | CSS animations / Framer Motion / Web Animations API               |
| `react-native-gesture-handler`                            | Touch gestures                | Pointer Events / touch-action CSS                                 |
| `react-native-svg`                                        | SVG rendering                 | Native `<svg>` / lucide-react (already done)                      |
| `hls.js`                                                  | Video playback                | hls.js (already done in web)                                      |
| `burnt` (toasts)                                          | Toast notifications           | sonner (already done)                                             |
| `qrcode`                                                  | QR display                    | qrcode (web-compatible)                                           |
| `frimousse`                                               | Emoji picker                  | frimousse (already done, works on web)                            |
| `react-native-sortables`                                  | Drag-to-reorder               | @hello-pangea/dnd (already done)                                  |
| `reanimated-color-picker`                                 | Color picker                  | Native `<input type="color">` or react-colorful                   |
| `tus-js-client`                                           | VOD upload                    | tus-js-client (web-compatible)                                    |
| `ua-parser-js`                                            | User-agent parsing            | Inline parser (already done in web)                               |
| `chrono-node`                                             | Date parsing                  | chrono-node (web-compatible)                                      |
| `sdp-transform`                                           | SDP parsing                   | sdp-transform (web-compatible)                                    |
| `@atproto/oauth-client-expo`                              | OAuth (RN)                    | @atproto/oauth-client-browser (already done)                      |
| `expo-crypto`                                             | Crypto                        | Web Crypto API                                                    |
| `@atproto/crypto`                                         | Identity                      | @atproto/crypto (web-compatible via webcrypto)                    |
| `viem`                                                    | Blockchain                    | viem (web-compatible)                                             |
| `multiformats`                                            | CID handling                  | multiformats (web-compatible)                                     |
| `jose`                                                    | JWT                           | jose (web-compatible)                                             |
| `expo-linking`                                            | Deep links                    | URL / window.location                                             |
| `react-native-webview`                                    | In-app browser                | iframe or popup window                                            |
| `expo-web-browser`                                        | OAuth browser                 | @atproto/oauth-client-browser (already done)                      |
| `@sentry/react-native`                                    | Error tracking                | @sentry/react                                                     |
| `react-native-edge-to-edge`                               | Edge-to-edge UI               | CSS safe-area-inset-\*                                            |
| `react-native-safe-area-context`                          | Safe area                     | CSS env(safe-area-inset-\*)                                       |
| `expo-splash-screen`                                      | Splash screen                 | CSS loading state                                                 |
| `expo-updates`                                            | OTA updates                   | N/A (web is always latest)                                        |
| `@react-native-firebase/app`                              | Firebase base                 | N/A                                                               |
| `expo-localization`                                       | Locale detection              | navigator.language / Intl                                         |
| `expo-system-ui`                                          | System UI                     | N/A                                                               |

### API Usage: Web-Only vs RN-Only

**APIs used in web but not RN:**

- `agent.place.stream.multistream.listTargets` / `putTarget` / `createTarget` / `deleteTarget` (RTMP target management)
- `agent.place.stream.server.listWebhooks` / `updateWebhook` / `createWebhook` / `deleteWebhook`
- `agent.place.stream.badge.def.list` / `def.create` / `issuance.create` (badge issuer)
- `agent.place.stream.branding.updateBlob` / `deleteBlob`
- `agent.place.stream.server.getStorage` / `upsertStorage`

**APIs used in RN but not web:**

- `agent.place.stream.media.getUploadStatus` / `createUpload` / `publishVideo` (upload flow)
- Grid-based multi-stream viewing (not RTMP management)

**Shared APIs (both use):**

- `agent.place.stream.live.getLiveUsers` (home page)
- `agent.place.stream.live.searchActorsTypeahead` (search/recommendations)
- `agent.place.stream.media.getVideoList` (video listings)
- Chat APIs (messages, pinned comments, mentions)

---

## The Plan

### Phase 1: Foundation & Modernization

The web app works but has rough edges: hand-rolled data fetching, missing providers, hardcoded theme, no error boundaries. This phase cleans up the foundation so everything that comes after is easier to build and maintain.

#### 1a. React Query

Add `@tanstack/react-query` and migrate data fetching incrementally.

**Why incremental:** A big-bang migration is high-risk and creates a massive diff. Incremental lets us convert one hook at a time, verify it works, then move on. The inconsistency is temporary and manageable.

**What to migrate first:** Start with the ugliest hand-rolled fetching. `useVideoList` is a good candidate - it manually manages loading, error, cursor, inFlight ref, and page accumulation. React Query's `useInfiniteQuery` replaces all of that.

**Migration order:**

1. `useVideoList` → `useInfiniteQuery` (cursor-based pagination)
2. `fetchLiveUsers` in `StreamplaceProvider` → `useQuery` with 5s refetch interval
3. Profile fetching (`getProfile`, `getProfiles`) → `useQuery` with stale-while-revalidate
4. Stream key records (`getStreamKeyRecords`) → `useQuery`
5. Chat profile (`getChatProfileRecordFromPDS`) → `useQuery`
6. Server settings (`getServerSettingsFromPDS`) → `useQuery`
7. Recommendations → `useQuery`

**What stays in Zustand:** Auth state, session, sidebar state, and anything that's not a server fetch. Zustand is for client state; React Query is for server state. The distinction matters.

**Provider mount:** Wrap the app in `QueryClientProvider` in `main.tsx`.

#### 1b. Error Boundary

Add a React error boundary at the router level. Catches render errors, shows a fallback UI with a retry button. Integrate with `@sentry/react` for error reporting (see 1g).

#### 1c. Suspense Fallbacks

Add `<Suspense>` with loading skeletons at route level. The home page already has skeleton placeholders for the stream grid; extend that pattern to other routes.

#### 1d. Mount Missing Providers

These already exist in `js/web/src/contexts/` but aren't wired into `main.tsx`:

- **`TooltipProvider`** - Mount in `main.tsx`. Required by the shadcn tooltip component.
- **`FullscreenProvider`** - Mount in `main.tsx` or in the player wrapper. Exposes `useFullscreen()` hook.
- **`VideoElementProvider`** - Mount in the player wrapper. Exposes `useVideoElement()` hook. Removes prop-passing in player components.

#### 1e. Theme Provider

Replace the hardcoded `"dark"` in `useColorScheme.ts` with a real implementation:

- Listen to `prefers-color-scheme` media query
- Support manual toggle (light/dark/system)
- Persist preference to localStorage
- CSS variables are already partially defined in `styles.css` for both modes

This doesn't need to be a full feature yet - just get the plumbing right so light mode works when we need it.

#### 1f. i18n Audit

Audit all hardcoded English strings in the web app. Add missing keys to `@streamplace/i18n` Fluent files. Focus on: chat chrome, player controls, header, home empty state, error messages.

#### 1g. Sentry

Add `@sentry/react` for error tracking. Initialize in `main.tsx`. Configure source maps upload in the Vite build.

#### 1h. Code Review & Cleanup

- Audit the zustand store slices. Some state may be redundant with React Query (e.g., `liveUsers` in the streamplace slice could become a React Query cache).
- Remove dead code and unused imports.
- Ensure consistent error handling patterns across slices.
- Document the state management split: Zustand for client state, React Query for server state.

---

### Phase 2: Feature Parity (No Go-Live)

These features bring the web app to parity with the RN app, minus the go-live flow and server-dependent features.

#### 2a. Chat Popout

New route: `/chat-popout/$user`

Standalone chat window with no player. `window.open()` from the chat sidebar "pop out" link. WebSocket connection to the chat stream. Minimal chrome - just the chat panel.

The `ChatPanel` component already exists and works. This is mostly a routing + layout task.

#### 2b. Block/Mute in Chat

Un-stub `createBlockRecord` in `blueskySlice.ts`. Wire `userMute` and `chatWarn` actions to the chat UI.

- Mute button on chat messages (hover or context menu)
- Block button on user hover card
- The `HoverCard` for chat users already exists in `chat-panel.tsx`

#### 2c. Chat Profile Edit

Route or modal for editing chat profile (color, avatar, badges). Wire `createChatProfileRecord` and `getChatProfileRecordFromPDS` - both are already implemented in the slice, they just need UI.

Currently the chat profile links out to Bluesky. The web app could have an inline editor using `<input type="color">` for the color picker.

#### 2d. VOD Download

Add a download button to the VOD player. Use `fetch` + `Blob` + `URL.createObjectURL` + `<a download>`. Simple feature, high utility.

#### 2e. Multi-Stream Viewer

New route: `/multi`

Grid of multiple simultaneous live streams. Use existing `VideoSection` components in a grid layout. Query params for which streams to show.

#### 2f. Search

New route: `/search`

Search bar in the header (when logged in). Search actors, streams, VODs. Leverage `place.stream.live.searchActorsTypeahead` (already used in recommendations).

#### 2g. Upload VOD

New route: `/upload`

`tus-js-client` for resumable uploads (already web-compatible). File picker via `<input type="file" accept="video/*">`. Upload progress, validation, metadata editor.

Port the logic from RN `upload.tsx` (1484 lines) but the web version will be simpler: no expo-document-picker, no expo-file-system, just native file input + tus.

#### 2h. Embed Screens

Routes: `/embed/$user`, `/embed/$user/video/$tid`, `/embed/info-widget/$user`, `/embed/danmu-obs/$user`

Minimal chrome, auto-play, no sidebar/header. Query params for configuration (chat visible, quality, etc.). These are important for distribution - streamers embed their streams on personal websites.

#### 2i. Popout Windows

Routes for popout views (livestream, multistream, stream monitor, info widget). Standalone windows with minimal UI. Lower priority than embeds but useful for power users.

#### 2k. About, Download, Support

These don't need to be full routes. They can be:

- Links in the sidebar footer or header
- External links to stream.place pages
- Or simple inline content accessible from settings

The functionality should be accessible without cluttering the route tree.

---

### Phase 3: Go-Live & Broadcaster Flow

The biggest gap. The RN app has a complete go-live flow; the web app has zero. This phase covers both OBS-based (RTMP) and browser-based (WebRTC) broadcasting.

#### 3a. Stream Key Creation

Un-stub `createStreamKeyRecord` in `blueskySlice.ts`. Add `@atproto/crypto` + `viem` to web dependencies. Build key generation flow using `@atproto/crypto` for keypair generation. Wire into `settings/keys.tsx` (currently read-only delete).

#### 3b. Go-Live Route & UI

New route: `/go-live`

"Start streaming" button in sidebar (replace disabled Following with Go Live when logged in). Stream key display + copy-to-clipboard. OBS/Restream setup instructions. RTMP URL + stream key display.

This covers both `launch-go-live.tsx` (the "Ready to go live?" prompt) and the go-live entry point from the RN app.

#### 3c. Live Dashboard

New route: `/$user/dashboard` (broadcaster view)

Un-stub `createLivestreamRecord` and `updateLivestreamRecord` in `blueskySlice.ts`. Implement `golivePost` in `blueskySlice.ts`.

**Dashboard vision:** Twitch-style dashboard with movable widgets. BentoGrid layout with drag-and-drop rearrangement. Widgets: self-view, chat, viewer count, stream health, multistream status, activity feed.

- Use `@dnd-kit/core` or similar for drag-and-drop widget arrangement
- Horizontal scroll support for widget overflow
- Widget state persisted to localStorage (positions, sizes)
- Keep-awake via `navigator.wakeLock.request("screen")`
- Mobile web: `<video autoplay muted playsinline>` for self-preview

The RN app uses `BentoGrid` from `@streamplace/components` which is RN-specific. The web implementation should be its own thing, built for the web-native drag-and-drop experience.

#### 3d. WebRTC Publishing (Web Broadcaster)

`getUserMedia()` for camera/mic capture. `RTCPeerConnection` for WHEP/WHIP publishing. This is the web-native equivalent of `react-native-webrtc` + `rtcaudiodevice`. Integrate with the live dashboard self-view.

This is significant work and only matters for a subset of users who want browser-based broadcasting. Could be deferred to after initial go-live if OBS-only (RTMP key + instructions) is sufficient.

Note: The RN app has `mobile-go-live.tsx` for mobile camera ingest. On web, this could be a simpler route or part of the go-live flow using `getUserMedia()` + WebRTC, which works on mobile browsers.

#### 3e. Following Feed

Sidebar "Following" link → working route. Fetch following list from PDS, filter live users to followed accounts, display as StreamCard grid (same as home, filtered).

This may need server-side work depending on how following data is fetched, plus optimisations and caching.

---

### Phase 4: Web-Native Enhancements

These go beyond parity to make the web app genuinely better than the RN app on desktop.

#### 4a. Picture-in-Picture ✅

`video.requestPictureInPicture()` API. PiP button in player controls. Continue watching while browsing other tabs.

#### 4b. Theatre Mode ✅

Sidebar + header hidden, video fills viewport. Toggle button in player controls, `T` keyboard shortcut. State persisted to localStorage.

#### 4c. Web Share API

Share button on streams and VODs. `navigator.share({ title, url, text })`. Fallback to copy-to-clipboard.

#### 4d. Keyboard Shortcuts

Global shortcuts: `G` for go-live, `/` for search, `?` for help. Player shortcuts: already done (space, m, f, T). Chat shortcuts: `Enter` to send, `Escape` to close.

#### 4e. Multi-Window / Pop-Out Player

Pop-out player to separate browser window. `window.open()` with minimal player UI. Sync play/pause state between windows.

#### 4f. URL-Based Deep Linking ✅

Every view has a stable URL (already done with TanStack Router). Shareable links to specific moments in VODs. Embeddable via query params.

#### 4g. Theming ✅

Full light mode + dark mode implementation. `prefers-color-scheme` media query listener. Manual toggle in settings. CSS variables already partially defined in `styles.css`.

#### 4h. PWA (Progressive Web App)

`manifest.json` with app name, icons, theme color. Service worker for offline shell caching. "Add to Home Screen" prompt. Offline fallback page.

---

## Decisions

### 1. React Query Adoption Scope

**Decision: Incremental.** Migrate one hook at a time, starting with the ugliest hand-rolled fetching. The inconsistency is temporary. A big-bang migration is high-risk and creates a massive diff that's hard to review.

### 2. WebRTC Publishing Priority

**Decision: Defer to after initial go-live.** OBS-only with RTMP key + instructions gets 90% of broadcasters. WebRTC publishing is significant work for a subset of users. Ship the basic go-live first, add WebRTC later.

### 3. Route Structure

**Decision: Add top-level routes for major features.** `/following`, `/search`, `/upload`, `/multi`, `/chat-popout/$user`. Keep settings nested. Embed routes under `/embed/`. Don't nest new features under existing sections just because the RN app does - the web has a sidebar, not tabs.

### 4. Pop-Out Windows

**Decision: Medium priority.** Embeds are higher priority (distribution). Pop-outs are nice for power users but most won't use them. Build embeds first, pop-outs second.

### 5. Upload Flow

**Decision: After go-live.** The go-live flow is the critical gap. Upload is important but secondary. When we do build it, `tus-js-client` + `<input type="file">` is simpler than the RN version.

### 6. Shared Package Strategy

**Decision: (c) Both.** Move shared logic to `@streamplace/core` (it's already well-structured). Keep web-specific UI in `js/web/src/`. The core package's livestream store and VOD interactions are already platform-agnostic; continue that pattern.

### 7. State Management Split

**Decision: Zustand for client state, React Query for server state.** Auth, session, sidebar, UI state stays in Zustand. Server data (live users, profiles, video lists, settings) moves to React Query. The distinction: if it's fetched from an API, it's React Query. If it's local UI state, it's Zustand.

---

## Progress

### Phase 1: Foundation — Complete

| Item                   | Status | Notes                                                                                                                                          |
| ---------------------- | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| 1a. React Query        | Done   | `@tanstack/react-query` in main.tsx. `useVideoList` migrated to `useInfiniteQuery`. `fetchLiveUsers` migrated to `useQuery` with 30s interval. |
| 1b. Error boundary     | Done   | React error boundary at router level with fallback UI and retry.                                                                               |
| 1c. Suspense fallbacks | Done   | Route-level Suspense with loading skeletons.                                                                                                   |
| 1d. Mount providers    | Done   | TooltipProvider, FullscreenProvider, VideoElementProvider mounted.                                                                             |
| 1e. Theme provider     | Done   | Media query listener, manual toggle, localStorage persistence.                                                                                 |
| 1f. i18n audit         | Done   | Keys added to en-US common.ftl. Stream/video components updated.                                                                               |
| 1g. Sentry             | Done   | `@sentry/react` initialized in main.tsx, source maps in Vite build.                                                                            |
| 1h. Code review        | Done   | Zustand slices audited, dead code removed.                                                                                                     |

### Phase 2: Feature Parity — In Progress

| Item                       | Status  | Notes                                                                                                                          |
| -------------------------- | ------- | ------------------------------------------------------------------------------------------------------------------------------ |
| 2a. Chat popout            | Done    | `/chat-popout/$user` route, standalone window, WebSocket chat.                                                                 |
| 2b. Block/mute in chat     | Pending | `createBlockRecord` still stubbed. UI buttons not wired.                                                                       |
| 2c. Chat profile edit      | Done    | Inline editor with color picker, avatar, badges.                                                                               |
| 2d. VOD download           | Done    | Fetch + Blob + createObjectURL + anchor download.                                                                              |
| 2e. Multi-stream viewer    | Pending | Not started. `multi.tsx` in RN shows grid of concurrent streams; web has nothing for this.                                     |
| 2f. Search                 | Done    | Inline header bar with popover results.                                                                                        |
| 2g. Upload VOD             | Done    | `/dashboard/upload` route with tus upload, metadata form, thumbnail, progress, video management. Moved into creator dashboard. |
| 2h. Embed screens          | Done    | `/embed/$user`, `/embed/$user/video/$tid`, `/embed/info-widget/$user`, `/embed/danmu-obs/$user`.                               |
| 2i. Popout windows         | Pending | Missing: popout-livestream, popout-multistream, popout-stream-monitor, popout-info-widget.                                     |
| 2k. About/Download/Support | Pending | About: web has settings/about but no public landing page. Download + Support completely missing.                               |

### Additional Work Done (not in original plan)

- Reply + pin message actions in chat
- Danmu (bullet comments) overlay with mobile-parity implementation
- Danmu toggle persisted to localStorage, standalone button in player controls
- Migrated fetchLiveUsers from zustand to React Query
- VOD upload with tus-js-client, metadata form, thumbnail, progress tracking

### Phase 3: Go-Live — In Progress

| Item                    | Status  | Notes                                                                                                      |
| ----------------------- | ------- | ---------------------------------------------------------------------------------------------------------- |
| 3a. Stream Key Creation | Done    | `createStreamKeyRecord` in blueskySlice with @atproto/crypto + viem. Key list + create in Stream Settings. |
| 3b. Go-Live Route & UI  | Pending | Not started.                                                                                               |
| 3c. Live Dashboard      | Pending | Not started.                                                                                               |
| 3d. WebRTC Publishing   | Pending | Deferred.                                                                                                  |
| 3e. Following Feed      | Pending | Not started.                                                                                               |

### Phase 4: Web-Native Enhancements — In Progress

| Item                       | Status  | Notes                                                                                                                                                                         |
| -------------------------- | ------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 4a. Picture-in-Picture     | Done    | PiP toggle button in player controls. Uses `requestPictureInPicture()` API. Support detection, state tracking via events.                                                     |
| 4b. Theatre mode           | Done    | Sidebar + header hidden, video fills viewport. Toggle button in player controls, `T` keyboard shortcut. State persisted to localStorage. Works on both live streams and VODs. |
| 4c. Web Share API          | Pending | Not started.                                                                                                                                                                  |
| 4d. Keyboard Shortcuts     | Partial | Player shortcuts done (space, m, f, T). Global shortcuts (`G`, `/`, `?`) not implemented.                                                                                     |
| 4e. Multi-Window/Pop-Out   | Pending | Not started.                                                                                                                                                                  |
| 4f. URL-Based Deep Linking | Done    | TanStack Router provides stable URLs for all routes.                                                                                                                          |
| 4g. Theming                | Done    | Media query listener, manual toggle, localStorage persistence.                                                                                                                |
| 4h. PWA                    | Pending | Not started.                                                                                                                                                                  |
