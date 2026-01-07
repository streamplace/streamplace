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
