# Settings Page Translations - English (US)

## App Version
app-version = Streamplace v{ $version }
download-new-update = Download New Update
check-for-updates = Check for Updates

bundled-runtype = Bundled
ota-runtype = Over-the-Air (OTA)
recovery-runtype = Recovery Mode

modal-latest-version = You are using the latest version.
modal-no-update-available = You are on the latest version of Streamplace, hooray!
modal-update-available-title = Update Available
modal-update-available-description = A new version of Streamplace is ready to download
modal-update-failed = Update check failed. You may need to update the app through the { $store }.
modal-update-failed-title = Update Failed
modal-update-failed-description = Update check failed. You may need to update the app through the { $store }.
button-reload-app-on-update = Apply Update (will reload app)

## Custom Node Settings
use-custom-node = Use Custom Node
default-url = Default: { $url }
enter-custom-node-url = Enter custom node URL
save-button = SAVE

## Language Settings
language-selection = Language
language-selection-description = Choose your preferred language
input-search-languages = Search languages...
help-translate = Help us translate Streamplace
help-translate-description = We're looking for volunteers to help translate Streamplace into more languages. If you're interested, please reach out to us on Discord or GitHub!
currently-translating = Translations are on the way
currently-translating-description = Some parts of the app may look incomplete. Thank you for your patience!

## Debug Recording
debug-recording-title = Allow { $host } to record your livestream for debugging and improving the service
debug-recording-description = Optional

## Key Management
manage-keys = Manage Keys

## Settings Page Specific
settings-title = Settings

## Navigation Categories
about = About
account = Account
advanced = Advanced
danmu = Danmu
developer = Developer
languages = Languages
privacy-security = Privacy & Security
streaming = Streaming

## Common Actions
cancel = Cancel
create = Create
delete = Delete
refresh = Refresh
save-button = Save
sign-in = Sign In
update = Update
log-out = Log out

## Account Settings
account-greeting = Hey, @{ $handle }.
edit-profile-bluesky = Edit profile (on Bluesky)
change-name-color = Change name color

## Key Management
key-management = Key Management
key-manager = Key Manager
manage-keys = Manage Keys
your-stream-pubkeys = Your Stream Public Keys
no-keys = No keys configured
pubkey-description = Public keys are paired with stream keys (used in streaming software) to sign and verify your stream
keys-count = { $count ->
    [one] { $count } key
   *[other] { $count } keys
}

## Recommendations
recommendations = Recommendations
manage-recommendations = Manage Recommendations
recommendations-to-others = Recommendations to Others
recommendations-description = Share up to 8 streamers you recommend to your viewers
no-recommendations-yet = No recommendations configured yet
add-recommendation = Add Recommendation
streamer-did = Streamer DID
recommendations-count = { $count ->
    [one] { $count } recommendation
   *[other] { $count } recommendations
}

## Webhook Management
webhooks = Webhooks
webhook-integrations = Webhook Integrations
webhook-integrations-description = Connect external services to receive real-time updates about your streams
create-webhook = Create Webhook
edit-webhook = Edit Webhook
delete-webhook = Delete Webhook
no-webhooks-yet = No webhooks configured yet
failed-load-webhooks = Failed to load webhooks
webhook-will-no-longer-receive-events = This webhook will no longer receive events
create-first-webhook-description = Create your first webhook to start receiving stream events
example-captain-hook = Captain Hook
webhooks-count = { $count ->
    [one] { $count } webhook
   *[other] { $count } webhooks
}

## Webhook Events
activates-on = Activates on:
events = Events
events-livestream = Livestream Events
events-chat = Chat Events
untitled-webhook = Untitled Webhook
inactive = Inactive

## Debug Recording
debug-recording = Debug Recording

## Danmu Settings
danmu = Danmu
danmu-enabled = Enable Danmu
danmu-enabled-description = Display live chat messages as floating comments on your screen
danmu-opacity = Opacity
danmu-speed = Speed
danmu-lane-count = Lane Count
danmu-max-messages = Max Messages

## General
app-version-description = No updates are available at this time
confirm-delete = Are you sure you want to delete this?
action-cannot-be-undone = This action cannot be undone
name-optional = Name (optional)
deleting = Deleting...
saving = Saving...
go-to-dashboard = Go to Dashboard
need-setup-live-dashboard = Need to set up streaming first? Visit the live dashboard
no-languages-found = No languages found
