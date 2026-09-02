# Common UI Translations - English (US)

## General UI
loading = Loading...
error = Error
cancel = Cancel
confirm = Confirm
close = Close
open = Open
ok = OK
yes = Yes
no = No
continue = Continue
back = Back
next = Next
finish = Finish

## Actions
save = Save
delete = Delete
edit = Edit
create = Create
update = Update
refresh = Refresh

## Status Messages
success = Success
warning = Warning
info = Information

## Input Placeholders
search-placeholder = Search...
search-for-streamers = Search for streamers...
message-input = Enter your message...

## Authentication & Access
please-log-in-to-access-this-page = Please log in to access this page
go-to-settings = Go to Settings
go-back = Go Back
sign-in = Sign In
log-in = Log in
sign-up = Sign up
go-home = Go home
try-again = Try again
completing-sign-in = Completing sign-in…
already-logged-in = You're already logged in.
signed-in-as = Signed in as @{ $handle }
signed-in-as-code = Signed in as { $handle }
sign-in-description = Sign in with your Atmosphere account, such as your Bluesky account.
handle-label = Handle
redirecting = Redirecting…
sign-in-failed = Sign-in failed

## Demo and Testing
welcome-user = Welcome, { $username }!
notification-count = { $count ->
    [0] No notifications
    [1] One notification
   *[other] { $count } notifications
}

## Offline User
user-offline = user is offline
user-offline-message = { $source ->
    [streamer] Looks like <1>@{ $handle } is offline</1>, but they recommend checking out:
   *[default] Looks like <1>@{ $handle } is offline</1>, but we recommend checking out:
}
user-offline-no-recommendations =
  Looks like <1>@{ $handle } is offline</1> right now.
  Check back later.
streaming-title = streaming { $title }
viewer-count = { $count ->
    [0] 0 viewers
    [1] 1 viewer
   *[other] { $count } viewers
}

## PDS Host Selector
pds-selector-title = New to the Atmosphere?
pds-selector-description = Choose where your Atmosphere account is hosted. You can use it with Bluesky, Streamplace, and other Atmosphere apps.
pds-selector-show-other-hosts = See { $count } other account hosts
pds-selector-custom-label = Another account host
pds-selector-custom-description = Enter your own account host URL
pds-selector-custom-url-label = Account host URL
pds-selector-custom-url-placeholder = https://pds.example.com
pds-selector-learn-more = Learn more about self-hosting
pds-selector-info = Each host has their own policies and reliability standards. Your account data lives on the host you choose, and you can migrate later.
pds-selector-different-policies = Streamplace has separate moderation policies. You can be banned from Streamplace regardless of which account host you choose.
pds-selector-read-policies = Read { $label }'s <tosLink>Terms of Service</tosLink> and <privacyLink>Privacy Policy</privacyLink> before continuing.
pds-selector-handle-policy-checkbox = I have read and agree to the <policyLink>handle policy</policyLink>
creator-not-found-title = Creator not found
creator-not-found-description = We couldn't find an Atmosphere account for @{ $handle }.
creator-lookup-failed-title = We couldn't load this creator
creator-lookup-failed-description = The Atmosphere directory isn't responding right now. Try again in a moment.

## Header
profile = Profile
more-results = More results

## Sidebar Navigation
nav-home = Home
nav-videos = Videos
nav-following = Following
nav-account = Account

## Chat
chat-no-messages = No messages yet
chat-scroll-to-top = Scroll to top
chat-scroll-to-bottom = Scroll to bottom
chat-reply-to-message = Reply to message
chat-pin-message = Pin message
chat-unpin-message = Unpin message
chat-dismiss-pinned = Dismiss pinned message
chat-view-profile-bluesky = View profile on Bluesky
chat-send-message = Send a message
chat-replying-to = Replying to { $handle }
chat-insert-emoji = Insert emoji
chat-skin-tone = Skin tone
chat-send-button = Chat
chat-failed-send = Failed to send message
chat-log-in-to = Log in to chat
chat-pop-out = Pop out chat
chat-close = Close chat
chat-open = Open chat
streamer-fallback = Streamer
view-profile = View profile

## Badges
badge-moderator = Moderator
badge-streamer = Streamer
badge-vip = VIP
badge-bot = Bot
badge-event = Event
badge-fallback = Badge
badge-issued-by-streamplace = Issued by Streamplace
badge-issued-by = Issued by { $issuer }
badge-issued = Issued
badge-self-labeled = Self-labeled
badge-details = Badge details
badge-back-to-profile = Back to profile
badge-previous = Previous badge
badge-next = Next badge
badge-count = Badge { $current } of { $total }

## Stream Info
stream-details = Stream details
activity-events = Events
activity-just-chatting = Just Chatting
activity-music = Music
activity-art = Art
activity-software-dev = Software Dev
activity-cooking = Cooking
activity-miniatures = Miniatures
activity-makers-crafting = Makers & Crafting
activity-fitness = Fitness
activity-sports = Sports
watching-count = { $count } watching
follow = Follow
share-copy = Copy
share-copied = Copied
share-copy-link = Copy Link
share-copy-embed-code = Copy Embed Code
share-copy-embed-url = Copy Embed URL
share-copy-failed = Couldn’t copy that
share-video = Share
share-video-message = Watch { $title } on Streamplace
share-to-bluesky = Share to Bluesky
share-more-options = More sharing options
share-failed = Couldn’t open sharing options
like-video = Like
unlike-video = Unlike
like-update-failed = Couldn’t update like
like-state-restored = Your previous like state was restored.
more-actions = More actions
video-tools = Video tools
video-embed = Embed
more-details = More details
less-details = Less details

## Video Player
offline = Offline
live-badge = Live
player-buffering = Buffering
reconnecting = Reconnecting…
stream-may-resume = The stream may resume shortly.
stream-offline = Stream offline
user-not-streaming = { $user } is not currently streaming
stream-is-offline-title = Stream is offline
user-not-streaming-check-back = { $user } is not currently streaming. Check back later.
back-to-home = Back to home
download-video = Download video
untitled = Untitled
default-stream-title = A livestream!
pip-enter = Picture-in-picture
pip-exit = Exit picture-in-picture
theatre-enter = Theatre mode
theatre-exit = Exit theatre mode

## Notifications
teleporting-in = Teleporting in

## Video Card
views-count = { $count ->
    [one] { $count } view
   *[other] { $count } views
}
likes-count = { $count ->
    [one] { $count } like
   *[other] { $count } likes
}

## Home Page
could-not-load-streams = Could not load streams. You might be offline.
live-now-count = { $count ->
    [one] { $count } person live now
   *[other] { $count } people live now
}
no-one-streaming = No one is streaming right now
check-back-later = Check back later?
hero-title = The video layer for everything.
hero-description = Streamplace is a streaming platform built on the AT Protocol.
learn-more = Learn more

## Videos Page
videos-title = Videos
no-videos-yet = No videos yet.
could-not-load-videos = Couldn't load videos: { $error }

## Search Page
no-results-for = No results for "{ $query }"
find-streamers = Find streamers by name or handle

## Error Boundary
something-went-wrong = Something went wrong
unexpected-error = An unexpected error occurred.

## Web App - Upload
upload-add-tag = Add tag, press Enter
upload-add-thumbnail = Add thumbnail image
upload-cancel = Cancel
upload-choose-file = Choose a video file
upload-choose-file-btn = Choose file
upload-content-warnings = Content Warnings
upload-delete = Delete
upload-deleting = Deleting...
upload-description = Description
upload-description-placeholder = Describe your video...
upload-details = Details
upload-file = File
upload-license = License
upload-optional-info = Optional · JPG, PNG up to 975KB
upload-preparing = Preparing upload...
upload-processing = Processing video
upload-publish = Publish
upload-published = Published
upload-publishing = Publishing...
upload-publishing-btn = Publishing...
upload-ready-publish = Ready to publish
upload-status = Status
upload-tags = Tags
upload-thumbnail = Thumbnail
upload-thumbnail-preview = Thumbnail preview
upload-title = Title
upload-title-placeholder = Give your video a title
upload-update-video = Update Video
upload-updating = Updating...
upload-view-video = View video →
upload-waiting-process = Waiting to process...

## Web App - Stream Key
create-key-description-delete-if-exposed = If you think your stream key has been exposed, delete it immediately and create a new key.
create-key-description-staff = We will never, ever, ask for your key.
create-key-do-not-share = DO NOT SHARE THIS KEY, including showing it on stream. Anyone in possession of your key may be able to stream from your account.
create-key-title = Are you sure about creating a new stream key?
create-stream-key = Create Stream Key
stream-key = Stream Key
stream-key-help = Your stream key identifies you to Streamplace. You can have more than one stream key at a time, and it's a good idea to prune old keys that you aren't using via the key manager.
stream-key-info-description = For security reasons, you won't be able to view it again through your account. If you lose this stream key, you'll need to generate a new one.
stream-key-label = Stream Key
stream-key-save-now = Please save this stream key somewhere safe.

## Web App - Metadata & Settings
allow-everyone-archive = Allow everyone to archive your content indefinitely
allow-everyone-distribute = Allow everyone to distribute your content
allowed-broadcasters = Allowed Broadcasters
allowed-broadcasters-help = Enter the DIDs of the broadcasters you want to allow, one per line.
content-rights = Content Rights
content-rights-help = Optional copyright and license information for your stream.
content-warnings = Content Warnings
content-warnings-help = You're required to flag your stream with themes that viewers may want a heads-up about.
copied-to-clipboard = Copied to clipboard
copy-link = Copy link
copyright-notice = Copyright Notice
copyright-year = Copyright Year
credit-line = Credit Line
custom-license = Custom…
custom-license-url = Custom License URL/Text
delete-after = Delete After
delete-after-help = Duration in seconds (e.g. 300 for 5 minutes).
distribution = Distribution
distribution-help = Control who can redistribute your stream and for how long archives are kept.
editing-video = Editing video
failed-to-create-key = Failed to create key
go-to-video = Go to video
key-manager = Key Manager
license = License
login-to-manage-videos = Please log in to manage your videos.
login-to-upload = Please log in to upload videos.
metadata = Metadata
metadata-learn-more = Learn more about content metadata
metadata-save-failed = Failed to save metadata
metadata-saved = Metadata saved
moderation = Moderation
moderation-coming-soon = Moderator management hooks aren't on the web yet — this section is a placeholder.
moderation-help = Add/remove stream moderators. Moderators can hide chat messages and time out users in your chat.
nav-dashboard = Dashboard
nav-settings = Settings
no-videos-yet-upload = No videos yet.
optional = Optional
required = Required
select-license = Select a license…

## Login
login-show-live-on-bluesky = Show when I'm live on Bluesky
login-show-live-on-bluesky-description = Gives your Bluesky avatar the red LIVE ring while you stream and lets Streamplace post announcements for you. Uncheck to sign in without granting any access to your Bluesky account.

## Video Player - Controls
player-play = Play
player-pause = Pause
player-mute = Mute
player-unmute = Unmute
player-seek = Seek
player-settings = Settings
player-fullscreen = Fullscreen
player-exit-fullscreen = Exit fullscreen
player-theatre = Theatre mode
player-exit-theatre = Exit theatre mode
player-pip = Picture-in-picture
player-exit-pip = Exit picture-in-picture
player-danmu = Danmu
player-enable-danmu = Enable danmu
player-disable-danmu = Disable danmu
player-quality = Quality
player-latency = Latency
player-quality-auto = Auto

## Video Player - Stats panel
player-stats = Player stats
player-stats-resolution = Resolution
player-stats-viewport = Viewport
player-stats-bitrate = Bitrate
player-stats-skipped = Skipped
player-stats-buffer = Buffer
player-stats-codecs = Codecs
player-stats-protocol = Protocol
player-stats-latency-mode = Latency Mode
player-stats-render-surface = Render Surface
player-stats-session = Session
player-protocol-webrtc = WebRTC
player-protocol-hls = HLS
player-latency-low = Low Latency
player-latency-standard = Standard

## Video Player - Errors
player-error-network = Network error
player-error-playback = Playback error
player-error-format = Stream format not supported
player-error-stream-not-live = Stream not live
player-error-network-retrying = Network error. Retrying.
player-error-media-recovering = Media error. Recovering.
player-error-hls-unsupported = Your browser doesn't support HLS playback.
player-error-stream-unavailable = Stream unavailable. Stopped reconnecting.
player-error-sdp-failed = Failed to create SDP offer
player-error-ice-failed = Failed to gather ICE candidates
player-error-negotiation-failed = WebRTC negotiation failed
player-error-stream-stalled = Stream stalled. Reconnecting.
player-error-connection-lost = Connection lost. Reconnecting.

## Upload
upload-error-format = File doesn't appear to be a supported video format (MP4, WebM, MKV, MOV, AVI, OGG, FLV, MPEG-TS).
upload-error-timeout = Processing timed out. Please try again later.
upload-error-processing = Processing failed
upload-error-not-logged-in = Not logged in
upload-title-untitled = Untitled

## Emoji picker
emoji-skin-tone = Skin tone
emoji-search = Search emoji…
