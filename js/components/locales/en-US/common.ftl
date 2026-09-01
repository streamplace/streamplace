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

## Streamer Agreement
streamer-agreement-title = One more thing.
streamer-agreement-intro = By using Streamplace, you agree to our community guidelines and terms of service. These include (but are not limited to) ensuring that you:
streamer-agreement-rule-1 = Comply with all applicable laws and regulations in both our and your area
streamer-agreement-rule-2 = Respect intellectual property rights and other lawful rights of others. Only stream content you have the right to broadcast. Do not dox others.
streamer-agreement-rule-3 = Maintain a respectful and harmonious environment for all users, and do not engage in harassment, hate speech, or acts that glorify terrorism or violent extremism.
streamer-agreement-rule-4 = Not stream content that is illegal, harmful, or violates our Terms of Service. <1>This includes pornographic content, and may include some graphic and sexual content.</1>  Do not stream content that is contrary to generally accepted community standards of decency and respect. <1>The official Streamplace node is not an adult platform.</1>
streamer-agreement-rule-5 = Not violate our policies. Doing so may result in the <1>removal of features available to you</1>  (including your ability to stream),  <1>account suspension, and in some cases, account termination.</1>
streamer-agreement-footer = For full details, please review our <1>Terms of Service</1> and <2>Community Guidelines</2>.
streamer-agreement-disclaimer = By clicking "Accept and Continue", you acknowledge that you have read and agree to the terms of service and community guidelines. If you do not agree, leave this app immediately.
streamer-agreement-accept = Accept and Continue
are-you-sure = Are you sure?

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
