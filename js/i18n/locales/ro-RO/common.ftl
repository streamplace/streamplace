# Common UI Translations - English (US)

## General UI
loading = Se încarcă...
error = Eroare
cancel = Anulare
confirm = Confirmare
close = Închidere
open = Deschidere
ok = OK
yes = Da
no = Nu
continue = Continuare
back = Înapoi
next = Următorul
finish = Finalizare

## Actions
save = Salvare
delete = Ștergere
edit = Editare
create = Creare
update = Actualizare
refresh = Reîmprospătare

## Status Messages
success = Succes
warning = Avertisment
info = Informație

## Input Placeholders
search-placeholder = Căutare...
message-input = Introduceți-vă mesajul...

## Authentication & Access
please-log-in-to-access-this-page = Vă rugăm să vă conectați pentru a accesa această pagină
go-to-settings = Mergeți la Setări
go-back = Mergeți înapoi

## Demo and Testing
welcome-user = Bun venit, { $username }!
notification-count = { $count ->
    [0] Nici o notificare
    [1] O notificare
   *[other] { $count } (de) notificări
}

## Offline User
user-offline = utilizatorul este offline
user-offline-message = { $source ->
    [streamer] Se pare că <1>@{ $handle } este offline</1>, dar recomandă să vizionați:
   *[default] Se pare că <1>@{ $handle } este offline</1>, dar vă recomandăm să vizionați:
}
user-offline-no-recommendations =
  Se pare că <1>@{ $handle } este offline</1> acum.
  Reveniți mai târziu.
streaming-title = transmisie în direct { $title }
viewer-count = { $count ->
    [0] 0 vizionări
    [1] 1 vizionare
   *[other] { $count } (de) vizionări
}

## PDS Host Selector
pds-selector-title = Nou pe Atmosphere?
pds-selector-description = Va trebui să selectați un PDS (Personal Data Server) pentru a accesa aplicațiile de pe Atmosphere, cum ar fi Bluesky, Tangled și Spark.
pds-selector-custom-label = Alt PDS
pds-selector-custom-description = Introduceți adresa URL a propriei gazde PDS
pds-selector-custom-url-label = URL PDS personalizat
pds-selector-custom-url-placeholder = https://pds.exemplu.ro
pds-selector-learn-more = Aflați mai multe despre găzduirea proprie
pds-selector-info = Fiecare gazdă are propriile politici și standarde de fiabilitate. Datele dvs. ATProto se află pe gazda pe care o alegeți și le puteți migra ulterior. Notă: Streamplace are propriile reguli de moderare - accesul dvs. poate fi interzis pe Streamplace indiferent de gazda pe care o alegeți.
pds-selector-read-policies = Citiți <tosLink>Termenii și condițiile</tosLink> și <privacyLink>Politica de confidențialitate</privacyLink> ale { $label } înainte de a continua.
pds-selector-handle-policy-checkbox = Am citit și sunt de acord cu <policyLink>politica de gestionare</policyLink>

## Login
login-show-live-on-bluesky = Arată când sunt live pe Bluesky
login-show-live-on-bluesky-description = Adaugă inelul roșu LIVE avatarului dvs. de Bluesky în timp ce transmiteți și permite Streamplace să publice anunțuri în numele dvs. Debifați pentru a vă conecta fără a acorda niciun acces la contul dvs. de Bluesky.
