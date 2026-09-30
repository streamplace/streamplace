# Common UI Translations - French (France)

## General UI
loading = Chargement...
error = Erreur
cancel = Annuler
confirm = Confirmer
close = Fermer
open = Ouvrir
ok = OK
yes = Oui
no = Non
continue = Continuer
back = Retour
next = Suivant
finish = Terminer

## Actions
save = Enregistrer
delete = Supprimer
edit = Modifier
create = Créer
update = Mettre à jour
refresh = Actualiser

## Status Messages
success = Succès
warning = Avertissement
info = Information

## Input Placeholders
search-placeholder = Rechercher...
message-input = Entrez votre message...

## Authentication & Access
please-log-in-to-access-this-page = Veuillez vous connecter pour accéder à cette page
go-to-settings = Aller aux Paramètres
go-back = Retour

## Demo and Testing
welcome-user = Bienvenue, { $username } !
notification-count = { $count ->
    [0] Aucune notification
    [1] Une notification
   *[other] { $count } notifications
}

## Offline User
user-offline = utilisateur hors ligne
user-offline-message = { $source ->
    [streamer] On dirait que <1>@{ $handle } est hors ligne</1>, mais ils recommandent de regarder :
   *[default] On dirait que <1>@{ $handle } est hors ligne</1>, mais nous recommandons de regarder :
}
user-offline-no-recommendations = 
  On dirait que <1>@{ $handle } est hors ligne</1> maintenant.
  Revenez plus tard.
streaming-title = diffusion de { $title }
viewer-count = { $count ->
    [0] 0 spectateurs
    [1] 1 spectateur
   *[other] { $count } spectateurs
}

## PDS Host Selector
pds-selector-title = Nouveau sur Atmosphere ?
pds-selector-description = Vous devrez sélectionner un PDS (Serveur de Données Personnel) pour accéder aux applications sur Atmosphere, comme Bluesky, Tangled et Spark.
pds-selector-custom-label = Un autre PDS
pds-selector-custom-description = Saisissez l'URL de votre propre hôte PDS
pds-selector-custom-url-label = URL PDS personnalisée
pds-selector-custom-url-placeholder = https://pds.exemple.com
pds-selector-learn-more = En savoir plus sur l'auto-hébergement
pds-selector-info = Chaque hôte a ses propres politiques et normes de fiabilité. Vos données ATProto se trouvent sur l'hôte que vous choisissez et vous pouvez migrer ultérieurement. Remarque : Streamplace a ses propres règles de modération — vous pouvez être banni de Streamplace quel que soit l'hôte choisi.
pds-selector-read-policies = Lisez les <tosLink>Conditions d'utilisation</tosLink> et la <privacyLink>Politique de confidentialité</privacyLink> de { $label } avant de continuer.
pds-selector-handle-policy-checkbox = J'ai lu et j'accepte la <policyLink>politique des identifiants</policyLink>

## Login
login-show-live-on-bluesky = Afficher quand je suis en direct sur Bluesky
login-show-live-on-bluesky-description = Ajoute l'anneau rouge LIVE à votre avatar Bluesky pendant vos streams et permet à Streamplace de publier des annonces pour vous. Décochez la case pour vous connecter sans accorder aucun accès à votre compte Bluesky.

## Stream notifications
teleporting-in = Téléportation dans
teleporting-to = Téléportation vers @{ $handle }

## Teleport dialog
teleport-dialog-title = Téléporter vers un autre streamer en direct
teleport-dialog-description = Choisissez un streamer vers lequel téléporter vos spectateurs.
teleport-unknown-streamer = Streamer inconnu
teleport-search-label = Rechercher les streams en direct
teleport-search-placeholder = Rechercher par identifiant ou titre
teleport-loading-streamers = Chargement des streamers en direct…
teleport-empty-streamers = Aucun streamer en direct trouvé.
teleport-no-matching-streamers = Aucun streamer en direct ne correspond.
teleport-viewer-count = { $count ->
    [one] { $count } spectateur
   *[other] { $count } spectateurs
}
teleport-countdown = Compte à rebours
teleport-seconds-range = secondes (5–300)
teleport-countdown-error = Le compte à rebours doit être compris entre 5 et 300 secondes.
teleport-selection-expired = Ce streamer n'est plus en direct. Choisissez-en un autre.
teleport-starting = Démarrage…
teleport-start = Téléporter
teleport-error-streamer-only = Seul le streamer du direct actuel peut lancer une téléportation.
teleport-error-handle-format = Saisissez un identifiant valide, par exemple handle.bsky.social.
teleport-error-countdown-number = Le compte à rebours doit être un nombre de secondes.
teleport-error-self = Vous ne pouvez pas vous téléporter vers vous-même.
teleport-error-resolve-handle = @{ $handle } est introuvable.
teleport-error-create = Impossible de démarrer la téléportation.
