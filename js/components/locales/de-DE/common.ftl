# Common UI Translations - German (DE)

## General UI
loading = Lädt...
error = Fehler
cancel = Abbrechen
confirm = Bestätigen
close = Schließen
open = Öffnen
ok = OK
yes = Ja
no = Nein
continue = Weiter
back = Zurück
next = Weiter
finish = Fertigstellen

## Actions
save = Speichern
delete = Löschen
edit = Bearbeiten
create = Erstellen
update = Aktualisieren
refresh = Aktualisieren

## Status Messages
success = Erfolg
warning = Warnung
info = Information

## Input Placeholders
search-placeholder = Suchen...
message-input = Geben Sie Ihre Nachricht ein...

## Authentication & Access
please-log-in-to-access-this-page = Bitte melden Sie sich an, um auf diese Seite zuzugreifen
go-to-settings = Zu den Einstellungen
go-back = Zurückgehen

## Demo and Testing
welcome-user = Willkommen, { $username }!
notification-count = { $count ->
    [0] Keine Benachrichtigungen
    [1] Eine Benachrichtigung
   *[other] { $count } Benachrichtigungen
}

## Offline User
user-offline = Benutzer ist offline
user-offline-message = { $source ->
    [streamer] Es scheint, als wäre <1>@{ $handle } offline</1>, aber es gibt folgende Empfehlungen:
   *[default] Es scheint, als wäre <1>@{ $handle } offline</1>, aber wir empfehlen Ihnen einen Blick auf:
}
user-offline-no-recommendations =
  Es scheint, als wäre <1>@{ $handle } offline</1>.
  Schauen Sie später wieder vorbei.
streaming-title = Streamt gerade { $title }
viewer-count = { $count ->
    [0] 0 Zuschauer
    [1] 1 Zuschauer
   *[other] { $count } Zuschauer
}

## PDS Host Selector
pds-selector-title = Neu in der Atmosphere?
pds-selector-description = Sie müssen einen PDS (Personal Data Server) auswählen, um auf Apps in der Atmosphere wie Bluesky, Tangled und Spark zuzugreifen.
pds-selector-custom-label = Ein anderer PDS
pds-selector-custom-description = Geben Sie die URL Ihres eigenen PDS-Hosts ein
pds-selector-custom-url-label = Benutzerdefinierte PDS-URL
pds-selector-custom-url-placeholder = https://pds.beispiel.de
pds-selector-learn-more = Erfahren Sie mehr über Self-Hosting
pds-selector-info = Jeder Host hat eigene Richtlinien und Zuverlässigkeitsstandards. Ihre ATProto-Daten verbleiben auf dem von Ihnen gewählten Host; ein späterer Umzug ist möglich. Hinweis: Streamplace hat eigene Moderationsregeln – Sie können von Streamplace ausgeschlossen werden, unabhängig davon, welchen Host Sie wählen.
pds-selector-read-policies = Lesen Sie die <tosLink>Nutzungsbedingungen</tosLink> und die <privacyLink>Datenschutzerklärung</privacyLink> von { $label }, bevor Sie fortfahren.
pds-selector-handle-policy-checkbox = Ich habe die <policyLink>Handle-Richtlinie</policyLink> gelesen und erkläre mich damit einverstanden.