# Settings Page Translations - German (DE)

## App Version
app-version = Streamplace v{ $version }
download-new-update = Update herunterladen
check-for-updates = Auf Updates prüfen

bundled-runtype = Gebündelt
ota-runtype = Over-the-Air (OTA)
recovery-runtype = Wiederherstellungsmodus

modal-latest-version = Sie verwenden die aktuellste Version.
modal-no-update-available = Sie nutzen die aktuellste Version von Streamplace!
modal-update-available-title = Update verfügbar
modal-update-available-description = Eine neue Version von Streamplace steht zum Download bereit.
modal-update-failed = Prüfung auf Updates fehlgeschlagen.
modal-update-failed-hint = Gegebenenfalls müssen Sie die App über den { $store } aktualisieren.
modal-update-failed-title = Update fehlgeschlagen
modal-update-failed-description = Die Suche nach Updates ist fehlgeschlagen.
button-reload-app-on-update = Update anwenden (App wird neu geladen)

## Custom Node Settings
use-custom-node = Benutzerdefinierten Node verwenden
default-url = Standard: { $url }
enter-custom-node-url = URL des benutzerdefinierten Nodes eingeben
save-button = SPEICHERN

## Language Settings
language-selection = Sprache
language-selection-description = Wählen Sie Ihre bevorzugte Sprache aus.
input-search-languages = Sprachen suchen...
help-translate = Helfen Sie uns bei der Übersetzung von Streamplace.
help-translate-description = Wir suchen Freiwillige, die uns helfen, Streamplace in weitere Sprachen zu übersetzen.
help-translate-contact = Bei Interesse kontaktieren Sie uns bitte auf Discord oder GitHub!
currently-translating = Übersetzungen sind in Arbeit
currently-translating-description = Einige Teile der App könnten unvollständig wirken.
patience-thanks = Vielen Dank für Ihre Geduld!

## Debug Recording
debug-recording-title = Erlauben Sie { $host }, Ihren Livestream zur Fehlerbehebung und Verbesserung des Dienstes aufzuzeichnen.
debug-recording-description = Optional

## Key Management
manage-keys = Schlüssel verwalten

## Settings Page Specific
settings-title = Einstellungen
error = Fehler

## Navigation Categories
about = Über
account = Konto
advanced = Erweitert
danmu = Danmu
developer = Entwickler
languages = Sprachen
privacy-security = Datenschutz & Sicherheit
streaming = Streaming

## Common Actions
cancel = Abbrechen
create = Erstellen
delete = Löschen
refresh = Aktualisieren
save-button = Speichern
sign-in = Anmelden
update = Aktualisieren
log-out = Abmelden
optional = optional

## Account Settings
account-greeting = Hallo, @{ $handle }.
edit-profile-bluesky = Profil bearbeiten (auf Bluesky)
change-name-color = Namensfarbe ändern

## Key Management
key-management = Schlüsselverwaltung
key-manager = Schlüssel-Manager
manage-keys = Schlüssel verwalten
your-stream-pubkeys = Ihre öffentlichen Stream-Schlüssel
no-keys = Keine Schlüssel konfiguriert
pubkey-description = Öffentliche Schlüssel werden mit Stream-Schlüsseln (in Ihrer Streaming-Software) gekoppelt, um Ihren Stream zu signieren und zu verifizieren.
keys-count = { $count ->
  [one] { $count } Schlüssel
  *[other] { $count } Schlüssel
}

## Backup Settings
backup = Backup
backup-enabled = S3-Backup
backup-enabled-description = Aufnahmen automatisch in S3-kompatiblem Speicher sichern
backup-connection-url = Verbindungs-URL
backup-connection-url-placeholder = s3+https://ACCESS_KEY:SECRET_KEY@s3.example.com/bucket
backup-endpoint = Endpunkt
backup-endpoint-placeholder = s3.beispiel.com
backup-bucket = Bucket
backup-bucket-placeholder = mein-backup-bucket
backup-access-key = Zugriffsschlüssel (Access Key)
backup-access-key-placeholder = AKIAIOSFODNN7BEISPIEL
backup-secret-key = Geheimer Schlüssel (Secret Key)
backup-secret-key-placeholder = wJalrXUtnFEMI/K7MDENG/bPxRfiCYBEISPIELSCHLUESSEL
backup-region = Region
backup-region-placeholder = us-east-1
backup-test-connection = Verbindung testen
backup-testing-connection = Verbindung wird getestet...
backup-connection-successful = Verbindung erfolgreich
backup-connection-failed = Verbindung fehlgeschlagen
show-password-in-url = Passwort in URL anzeigen
show-password-in-url-description = Den geheimen Schlüssel in der Verbindungs-URL anzeigen (falls eingegeben)
requested-seconds-per-segment = Sekunden pro Segment
requested-seconds-per-segment-description = Legen Sie die Sekunden pro Segment fest, die der Server verwenden soll.


## Recommendations
recommendations = Empfehlungen
manage-recommendations = Empfehlungen verwalten
recommendations-to-others = Empfehlungen für Andere
recommendations-description = Teilen Sie bis zu 8 Streamer, die Sie Ihren Zuschauern empfehlen.
no-recommendations-yet = Noch keine Empfehlungen konfiguriert
add-recommendation = Empfehlung hinzufügen
streamer-did = Streamer-DID
recommendations-count = { $count ->
  [one] { $count } Empfehlung
  *[other] { $count } Empfehlungen
}

## Webhook Management
webhooks = Webhooks
webhook-integrations = Webhook-Integrationen
webhook-integrations-description = Verbinden Sie externe Dienste, um Echtzeit-Updates über Ihre Streams zu erhalten.
create-webhook = Webhook erstellen
edit-webhook = Webhook bearbeiten
delete-webhook = Webhook löschen
no-webhooks-yet = Noch keine Webhooks konfiguriert
failed-load-webhooks = Laden der Webhooks fehlgeschlagen
webhook-will-no-longer-receive-events = Dieser Webhook wird keine Ereignisse mehr empfangen.
create-first-webhook-description = Erstellen Sie Ihren ersten Webhook, um Stream-Ereignisse zu empfangen.
example-captain-hook = Captain Hook
webhooks-count = { $count ->
  [one] { $count } Webhook
  *[other] { $count } Webhooks
}

## Webhook Events
activates-on = Aktiviert bei:
events = Ereignisse
events-livestream = Livestream-Ereignisse
events-chat = Chat-Ereignisse
untitled-webhook = Unbenannter Webhook
inactive = Inaktiv
active = Aktiv

## Multistreaming
multistream = Multistreaming
multistream-targets = Multistream-Ziele
multistream-description = Leiten Sie Ihre Streamplace-Livestreams automatisch an andere Streaming-Dienste wie Twitch oder YouTube weiter.
create-multistream-target = Multistream-Ziel erstellen
untitled-multistream-target = Unbenanntes Ziel
failed-load-multistream-targets = Laden der Multistream-Ziele fehlgeschlagen. Bitte versuchen Sie es erneut.
failed-toggle-multistream-target = Umschalten des Multistream-Ziels fehlgeschlagen. Bitte versuchen Sie es erneut.
failed-delete-multistream-target = Löschen des Multistream-Ziels fehlgeschlagen. Bitte versuchen Sie es erneut.
no-multistream-targets-yet = Noch keine Ziele vorhanden!
multistream-targets-count = { $count ->
  [one] { $count } Ziel
  *[other] { $count } Ziele
}
multistream-delete-target-confirmation = Sind Sie sicher, dass Sie "{ $target }" löschen möchten?
this-action-cannot-be-undone = Diese Aktion kann nicht rückgängig gemacht werden.
rtmp-target-name = RTMP-Ziel
rtmp-target-url = RTMP-URL
rtmp-target-name-placeholder = Mein Multistream-Ziel
multistream-create-target = Ziel erstellen
multistream-edit-target = Ziel bearbeiten
created = erstellt
status = Status

## Debug Recording
debug-recording = Debug-Aufzeichnung

## Danmu Settings
danmu = Danmu
danmu-enabled = Danmu aktivieren
danmu-enabled-description = Live-Chat-Nachrichten als fließende Kommentare auf Ihrem Bildschirm anzeigen
danmu-opacity = Deckkraft
danmu-speed = Geschwindigkeit
danmu-lane-count = Anzahl der Bahnen
danmu-max-messages = Maximale Nachrichtenanzahl

## General
app-version-description = Derzeit sind keine Updates verfügbar.
confirm-delete = Sind Sie sicher, dass Sie dies löschen möchten?
action-cannot-be-undone = Diese Aktion kann nicht rückgängig gemacht werden.
name-optional = Name (optional)
deleting = Wird gelöscht...
saving = Wird gespeichert...
go-to-dashboard = Zum Dashboard
need-setup-live-dashboard = Müssen Sie zuerst das Streaming einrichten?
visit-live-dashboard = Besuchen Sie das Live-Dashboard
no-languages-found = Keine Sprachen gefunden

## Branding Administration
branding = Branding
branding-admin = Branding-Administration
branding-admin-description = Passen Sie Ihre Streamplace-Instanz an.
branding-propagation-note = Beachten Sie, dass es einige Stunden dauern kann, bis die Einstellungen übernommen werden.
branding-login-required = Bitte melden Sie sich an, um das Branding zu verwalten.
branding-configuration = Konfiguration
branding-text-settings = Texteinstellungen
branding-colors = Farben
branding-legal-links = Rechtliche Links
branding-images = Bilder

## Branding Fields
branding-broadcaster-did = Broadcaster-DID
branding-broadcaster-did-description = Leer lassen, um den Server-Standard zu verwenden
branding-site-title = Seitentitel
branding-site-title-placeholder = Neuen Seitentitel eingeben
branding-site-description = Seitenbeschreibung
branding-site-description-placeholder = Seitenbeschreibung eingeben
branding-default-streamer = Standard-Streamer
branding-default-streamer-none = Keiner
branding-default-streamer-placeholder = did:plc:...
branding-clear-default-streamer = Standard-Streamer löschen
branding-primary-color = Primärfarbe
branding-primary-color-placeholder = #6366f1
branding-accent-color = Akzentfarbe
branding-accent-color-placeholder = #8b5cf6
branding-main-logo = Hauptlogo
branding-main-logo-description = SVG, PNG oder JPEG (max. 500KB)
branding-favicon = Favicon
branding-favicon-description = SVG, PNG oder ICO (max. 100KB)
branding-sidebar-bg = Hintergrundbild der Seitenleiste
branding-sidebar-bg-description = SVG, PNG oder JPEG (max. 500kb) – wird unten in der Seitenleiste über die volle Breite ausgerichtet. Laden Sie für beste Ergebnisse ein Bild mit Transparenz hoch, da es derzeit keine separate Deckkraft-Option gibt.
branding-current = Aktuell: { $value }
branding-dimensions = { $height } x { $width }

## Branding Actions
branding-upload-logo = Logo hochladen
branding-delete-logo = Logo löschen
branding-upload-favicon = Favicon hochladen
branding-delete-favicon = Favicon löschen
branding-upload-background = Hintergrund hochladen
branding-delete-background = Hintergrund löschen
branding-web-only = Bild-Uploads sind nur im Web verfügbar.

## Branding Legal Links
refresh-branding = Branding-Assets aktualisieren
branding-add-legal-link = Rechtlichen Link hinzufügen
branding-edit-legal-link = Rechtlichen Link bearbeiten
branding-legal-link-text-placeholder = Link-Text (z. B. Datenschutzerklärung)
branding-legal-link-url-placeholder = URL (z. B. https://beispiel.com/datenschutz)
add = Hinzufügen
edit = Bearbeiten

## Branding Toast Messages
branding-not-authenticated = Bitte melden Sie sich zuerst an
branding-empty-value = Bitte geben Sie einen Wert ein
branding-update-success = { $key } erfolgreich aktualisiert
branding-upload-success = { $key } erfolgreich hochgeladen
branding-delete-success = { $key } erfolgreich gelöscht
branding-upload-failed = Upload fehlgeschlagen
branding-delete-failed = Löschen fehlgeschlagen
branding-not-available = Datei-Uploads sind nur im Web verfügbar

## Navigation Categories (About Page)
node-legal-documents = Broadcaster-spezifische Dokumente

backup-save = Backup-Einstellungen speichern
backup-saving = Backup-Einstellungen werden gespeichert...
backup-secret-key-set-placeholder = (Passwort bereits gesetzt)
backup-error-invalid-endpoint = Muss ein gültiger Domainname sein
backup-error-invalid-bucket = Darf nur Kleinbuchstaben, Zahlen, Punkte und Bindestriche enthalten
backup-error-invalid-segment-duration = Muss zwischen 1 und 60 Sekunden liegen
backup-error-load-failed = Laden der Speichereinstellungen fehlgeschlagen
backup-error-update-failed = Aktualisieren des Backup-Status fehlgeschlagen
backup-error-save-failed = Speichern der Speichereinstellungen fehlgeschlagen
backup-error-missing-secret = S3-Einstellungen können ohne den geheimen Schlüssel nicht aktualisiert werden. Bitte geben Sie ihn erneut ein.
backup-segment-duration-placeholder = 6
backup-connection-url-placeholder = Muss eine gültige S3-URL im Format s3+https://ACCESS_KEY:SECRET_KEY@region.endpoi.nt/bucket sein