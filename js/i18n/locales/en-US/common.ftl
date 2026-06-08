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
message-input = Enter your message...

## Authentication & Access
please-log-in-to-access-this-page = Please log in to access this page
go-to-settings = Go to Settings
go-back = Go Back

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
pds-selector-description = You'll need to select a PDS (Personal Data Server) to access apps on the Atmosphere, such as Bluesky, Tangled, and Spark.
pds-selector-custom-label = Another PDS
pds-selector-custom-description = Enter your own PDS host URL
pds-selector-custom-url-label = Custom PDS URL
pds-selector-custom-url-placeholder = https://pds.example.com
pds-selector-learn-more = Learn more about self-hosting
pds-selector-info = Each host has their own policies and reliability standards. Your ATProto data lives on the host you choose and you can migrate later. Note: Streamplace has its own moderation rules - you can be banned from Streamplace regardless of which host you choose.
pds-selector-read-policies = Read { $label }'s <tosLink>Terms of Service</tosLink> and <privacyLink>Privacy Policy</privacyLink> before continuing.
pds-selector-handle-policy-checkbox = I have read and agree to the <policyLink>handle policy</policyLink>

## Login
login-show-live-on-bluesky = Show when I'm live on Bluesky
login-show-live-on-bluesky-description = Gives your Bluesky avatar the red LIVE ring while you stream and lets Streamplace post announcements for you. Uncheck to sign in without granting any access to your Bluesky account.
