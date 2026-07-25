# Settings Page Translations - French (France)

## App Version
app-version = Streamplace v{ $version }
download-new-update = Télécharger la nouvelle mise à jour
check-for-updates = Vérifier les mises à jour

bundled-runtype = Inclus
ota-runtype = Over-the-Air (OTA)
recovery-runtype = Mode de récupération

modal-latest-version = Vous utilisez la dernière version.
modal-no-update-available = Vous avez la dernière version de Streamplace, génial !
modal-update-available-title = Mise à jour disponible
modal-update-available-description = Une nouvelle version de Streamplace est prête à télécharger
modal-update-failed = La vérification des mises à jour a échoué. Vous devrez peut-être mettre à jour l'application via { $store }.
modal-update-failed-title = Échec de la mise à jour
modal-update-failed-description = La vérification des mises à jour a échoué. Vous devrez peut-être mettre à jour l'application via { $store }.
button-reload-app-on-update = Appliquer la mise à jour (l'application va se recharger)

## Custom Node Settings
use-custom-node = Utiliser un nœud personnalisé
default-url = Par défaut : { $url }
enter-custom-node-url = Saisir l'URL du nœud personnalisé
save-button = ENREGISTRER

## Language Settings
language-selection = Langue
language-selection-description = Choisissez votre langue préférée
help-translate = Aidez-nous à traduire Streamplace
help-translate-description = Nous recherchons des bénévoles pour aider à traduire Streamplace dans plus de langues. Si vous êtes intéressé, contactez-nous sur Discord ou GitHub !
currently-translating = Les traductions sont en cours
currently-translating-description = Certaines parties de l'application peuvent sembler incomplètes. Merci pour votre patience !

## Debug Recording
debug-recording-title = Autoriser { $host } à enregistrer votre diffusion en direct pour le débogage et l'amélioration du service
debug-recording-description = Optionnel
input-search-languages = Rechercher des langues...

## Key Management
manage-keys = Gérer les clés

## General UI
settings-title = Paramètres
loading = Chargement...
error = Erreur
cancel = Annuler
confirm = Confirmer

## Demo and Testing
welcome-user = Bienvenue, { $username } !
notification-count = { $count ->
    [0] Aucune notification
    [1] Une notification
   *[other] { $count } notifications
}
search-placeholder = Rechercher...
message-input = Saisissez votre message...

## Status Messages
success = Succès
warning = Attention
info = Information
close = Fermer
open = Ouvrir
delete = Supprimer
edit = Modifier
create = Créer
update = Mettre à jour
refresh = Actualiser

## Actions
save = Enregistrer
cancel-button = Annuler
ok = OK
yes = Oui
no = Non
continue = Continuer
back = Retour
next = Suivant
finish = Terminer

## Catégories de Navigation
about = À propos
account = Compte
advanced = Avancé
danmu = Danmu
developer = Développeur
languages = Langues
privacy-security = Confidentialité et Sécurité
streaming = Diffusion
notifications = Notifications

## Actions Courantes
cancel = Annuler
create = Créer
delete = Supprimer
refresh = Actualiser
save-button = Enregistrer
sign-in = Se connecter
update = Mettre à jour
log-out = Se déconnecter

## Paramètres du Compte
account-greeting = Salut, @{ $handle }.
edit-profile-bluesky = Modifier le profil (sur Bluesky)
change-name-color = Changer la couleur du nom

## Gestion des Clés
key-management = Gestion des Clés
key-manager = Gestionnaire de Clés
manage-keys = Gérer les Clés
your-stream-pubkeys = Vos Clés Publiques de Diffusion
no-keys = Aucune clé configurée
pubkey-description = Les clés publiques sont associées aux clés de diffusion (utilisées dans le logiciel de streaming) pour signer et vérifier votre diffusion
keys-count = { $count ->
    [one] { $count } clé
    [many] { $count } clés
   *[other] { $count } clés
}

## Gestion des Webhooks
webhooks = Webhooks
webhook-integrations = Intégrations de Webhook
webhook-integrations-description = Connectez des services externes pour recevoir des mises à jour en temps réel sur vos diffusions
create-webhook = Créer un Webhook
edit-webhook = Modifier le Webhook
delete-webhook = Supprimer le Webhook
no-webhooks-yet = Aucun webhook configuré pour le moment
failed-load-webhooks = Échec du chargement des webhooks
webhook-will-no-longer-receive-events = Ce webhook ne recevra plus d'événements
create-first-webhook-description = Créez votre premier webhook pour commencer à recevoir des événements de diffusion
example-captain-hook = Capitaine Crochet
webhooks-count = { $count ->
    [one] { $count } webhook
   *[other] { $count } webhooks
}

## Événements de Webhook
activates-on = S'active sur :
events-livestream = Événements de Diffusion en Direct
events-chat = Événements de Chat
untitled-webhook = Webhook Sans Titre
inactive = Inactif

## Enregistrement de Débogage
debug-recording = Enregistrement de Débogage

## Paramètres Danmu
danmu-enabled = Activer Danmu
danmu-enabled-description = Afficher les messages de chat en direct sous forme de commentaires flottants sur votre écran
danmu-opacity = Opacité
danmu-speed = Vitesse
danmu-lane-count = Nombre de Voies
danmu-max-messages = Messages Maximum

## Général
app-version-description = Informations sur la version actuelle
confirm-delete = Êtes-vous sûr de vouloir supprimer ceci ?
action-cannot-be-undone = Cette action ne peut pas être annulée
name-optional = Nom (optionnel)
deleting = Suppression...
saving = Enregistrement...
go-to-dashboard = Aller au Tableau de Bord
need-setup-live-dashboard = Vous devez d'abord configurer une diffusion en direct dans le tableau de bord
no-languages-found = Aucune langue trouvée
backup-save = Enregistrer les paramètres de sauvegarde
backup-saving = Enregistrement des paramètres de sauvegarde...
backup-secret-key-set-placeholder = (Mot de passe déjà défini)
backup-error-invalid-endpoint = Doit être un nom de domaine valide
backup-error-invalid-bucket = Doit contenir uniquement des lettres minuscules, des chiffres, des points et des traits d'union
backup-error-invalid-segment-duration = Doit être compris entre 1 et 60 secondes
backup-error-load-failed = Échec du chargement des paramètres de stockage
backup-error-update-failed = Échec de la mise à jour de l'état de la sauvegarde
backup-error-save-failed = Échec de l'enregistrement des paramètres de stockage
backup-error-missing-secret = Impossible de mettre à jour les paramètres S3 sans la clé secrète. Veuillez la saisir à nouveau.
backup-segment-duration-placeholder = 6

## Backup Settings
backup = Sauvegarde
backup-enabled = S3 Backup
backup-enabled-description = Sauvegarder automatiquement les enregistrements dans un stockage compatible S3
backup-connection-url = URL de connexion
backup-connection-url-placeholder = s3+https://ACCESS_KEY:SECRET_KEY@s3.example.com/bucket
backup-endpoint = Point de terminaison
backup-endpoint-placeholder = s3.example.com
backup-bucket = Compartiment
backup-bucket-placeholder = my-backup-bucket
backup-access-key = Clé d'accès
backup-access-key-placeholder = AKIAIOSFODNN7EXAMPLE
backup-secret-key = Clé secrète
backup-secret-key-placeholder = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
show-password-in-url = Afficher le mot de passe dans l'URL
show-password-in-url-description = Afficher la clé secrète dans l'URL de connexion (si saisie)

## Recommendations
recommendations-to-others = Recommandations aux autres
recommendations-description = Partagez jusqu'à 8 streameurs que vous recommandez à vos spectateurs
no-recommendations-yet = Aucune recommandation configurée pour le moment

## Multistreaming
multistream = Multidiffusion
multistream-targets = Cibles de multidiffusion
multistream-description = Envoyez automatiquement vos diffusions en direct Streamplace vers d'autres services comme Twitch ou YouTube.
create-multistream-target = Créer une cible de multidiffusion
untitled-multistream-target = Cible sans titre
failed-load-multistream-targets = Échec du chargement des cibles de multidiffusion. Veuillez réessayer.
failed-toggle-multistream-target = Échec du basculement de la cible de multidiffusion. Veuillez réessayer.
no-multistream-targets-yet = Aucune cible pour le moment !
multistream-targets-count = { $count ->
    [one] { $count } cible
    [many] { $count } cibles
   *[other] { $count } cibles
}
multistream-delete-target-confirmation = Êtes-vous sûr de vouloir supprimer "{ $target }" ?
this-action-cannot-be-undone = Cette action ne peut pas être annulée.
rtmp-target-name = Cible RTMP
rtmp-target-url = URL RTMP
rtmp-target-name-placeholder = Ma cible de multidiffusion
multistream-create-target = Créer la cible
multistream-edit-target = Modifier la cible
created = créé
status = statut

## Branding Administration
branding = Image de marque
branding-admin = Administration de l'image de marque
branding-admin-description = Personnalisez votre instance Streamplace. Notez que les paramètres peuvent prendre quelques heures à se propager.
branding-login-required = Veuillez vous connecter pour gérer l'image de marque
branding-configuration = Configuration
branding-text-settings = Paramètres de texte
branding-colors = Couleurs
branding-legal-links = Liens légaux
branding-images = Images

## Branding Fields
branding-broadcaster-did = DID du diffuseur
branding-broadcaster-did-description = Laisser vide pour utiliser la valeur par défaut du serveur
branding-site-title = Titre du site
branding-site-title-placeholder = Saisir un nouveau titre de site
branding-site-description = Description du site
branding-site-description-placeholder = Saisir la description du site
branding-default-streamer = Streameur par défaut
branding-default-streamer-none = Aucun
branding-default-streamer-placeholder = did:plc:...
branding-clear-default-streamer = Effacer le streameur par défaut
branding-primary-color = Couleur principale
branding-primary-color-placeholder = #6366f1
branding-accent-color = Couleur d'accent
branding-accent-color-placeholder = #8b5cf6
branding-main-logo = Logo principal
branding-main-logo-description = SVG, PNG ou JPEG (max 500 Ko)
branding-favicon = Favicon
branding-favicon-description = SVG, PNG ou ICO (max 100 Ko)
branding-sidebar-bg = Image d'arrière-plan de la barre latérale
branding-sidebar-bg-description = SVG, PNG ou JPEG (max 500 ko) - apparaît aligné en bas de la barre latérale, pleine largeur. Téléchargez une image avec opacité pour de meilleurs résultats, car il n'existe pas actuellement d'option d'opacité séparée.
branding-current = Actuel : { $value }
branding-dimensions = { $height } x { $width }

## Branding Actions
branding-upload-logo = Télécharger le logo
branding-delete-logo = Supprimer le logo
branding-upload-favicon = Télécharger le favicon
branding-delete-favicon = Supprimer le favicon
branding-upload-background = Télécharger l'arrière-plan
branding-delete-background = Supprimer l'arrière-plan
branding-web-only = Les téléchargements d'images ne sont disponibles que sur le web.

## Branding Legal Links
refresh-branding = Actualiser les ressources de l'image de marque
branding-add-legal-link = Ajouter un lien légal
branding-edit-legal-link = Modifier le lien légal
branding-legal-link-text-placeholder = Texte du lien (ex., Politique de confidentialité)
branding-legal-link-url-placeholder = URL (ex., https://exemple.com/confidentialite)
add = Ajouter
active = Actif
optional = optionnel

## Branding Toast Messages
branding-not-authenticated = Veuillez d'abord vous connecter
branding-empty-value = Veuillez saisir une valeur
branding-update-success = { $key } mis à jour avec succès
branding-upload-success = { $key } téléchargé avec succès
branding-delete-success = { $key } supprimé avec succès
branding-upload-failed = Échec du téléchargement
branding-delete-failed = Échec de la suppression
branding-not-available = Les téléchargements de fichiers ne sont disponibles que sur le web

## Navigation Categories (About Page)
node-legal-documents = Documents spécifiques au diffuseur

## Badges
badges = Badges
badges-cosmetic-section = Badges cosmétiques
badges-empty-state = Vous n'avez pas encore reçu de badges.
badges-failed-load = Échec du chargement des badges
badges-failed-update = Échec de la mise à jour de la sélection de badges
badges-issued-by = Délivré par { $issuer }
badges-streamer-section = Badges du streameur

## Issue Badges
issue-badges = Délivrer des badges
issue-badges-back-to-definitions = Définitions de badges
issue-badges-badge-name = Nom du badge
issue-badges-badge-name-placeholder = ex. VIP, Supporter
issue-badges-badge-type = Type de badge
issue-badges-choose-image = Choisir une image
issue-badges-create-definition = Créer une définition de badge
issue-badges-create-definition-description = Définissez un nouveau type de badge qui peut être délivré aux spectateurs
issue-badges-create-definition-subtitle = Définir un nouveau type de badge
issue-badges-definition-created = Définition de badge créée
issue-badges-description-optional = Description (optionnelle)
issue-badges-description-placeholder = Description optionnelle pour ce badge
issue-badges-failed-create = Échec de la création de la définition de badge
issue-badges-failed-issue = Échec de la délivrance du badge
issue-badges-failed-load = Échec du chargement des définitions de badges
issue-badges-image-optional = Image (optionnelle)
issue-badges-image-too-large = L'image doit faire moins de 256 Ko
issue-badges-image-web-only = Le téléchargement d'images n'est disponible que sur le web
issue-badges-issue-badge = Délivrer un badge
issue-badges-issue-badge-description = Délivrez le badge "{ $name }" à un spectateur par son DID
issue-badges-issued = Badge délivré
issue-badges-issued-to = Délivré à { $did }
issue-badges-manage-description = Créez des définitions de badges et délivrez-les aux spectateurs
issue-badges-recipient-did = DID du destinataire
issue-badges-recipient-did-placeholder = did:plc:...
issue-badges-tap-to-issue = Appuyer pour délivrer
issue-badges-your-definitions = Vos définitions de badges
