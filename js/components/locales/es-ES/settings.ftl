# Settings Page Translations - Spanish (Spain)

## App Version
app-version = Streamplace v{ $version }
download-new-update = Descargar Nueva Actualización
check-for-updates = Buscar Actualizaciones

bundled-runtype = Empaquetado
ota-runtype = Over-the-Air (OTA)
recovery-runtype = Modo de Recuperación

modal-latest-version = Estás usando la última versión.
modal-no-update-available = ¡Tienes la versión más reciente de Streamplace, genial!
modal-update-available-title = Actualización Disponible
modal-update-available-description = Una nueva versión de Streamplace está lista para descargar
modal-update-failed = La búsqueda de actualizaciones falló. Es posible que necesites actualizar
    la aplicación a través de { $store }.
modal-update-failed-title = Actualización Falló
modal-update-failed-description = La búsqueda de actualizaciones falló. Es posible que necesites actualizar
    la aplicación a través de { $store }.
button-reload-app-on-update = Aplicar Actualización (recargará la aplicación)

## Custom Node Settings
use-custom-node = Usar Nodo Personalizado
default-url = Predeterminado: { $url }
enter-custom-node-url = Introduce la URL del nodo personalizado
save-button = GUARDAR

## Language Settings
language-selection = Idioma
language-selection-description = Elige tu idioma preferido
help-translate = Ayúdanos a traducir Streamplace
help-translate-description = Buscamos voluntarios para ayudar a traducir Streamplace a más idiomas. Si te interesa, contáctanos en Discord o GitHub!
currently-translating = Las traducciones están en camino
currently-translating-description = Algunas partes de la aplicación pueden verse incompletas. ¡Gracias por tu paciencia!

## Debug Recording
debug-recording-title = Permitir que { $host } grabe tu retransmisión en directo para depuración y mejora del servicio
debug-recording-description = Opcional
input-search-languages = Buscar idiomas...

## Key Management
manage-keys = Gestionar Claves

## General UI
settings-title = Configuración
loading = Cargando...
error = Error
cancel = Cancelar
confirm = Confirmar

## Demo and Testing
welcome-user = ¡Bienvenido, { $username }!
notification-count = { $count ->
    [0] Sin notificaciones
    [1] Una notificación
   *[other] { $count } notificaciones
}
search-placeholder = Buscar...
message-input = Introduce tu mensaje...

## Status Messages
success = Éxito
warning = Aviso
info = Información
close = Cerrar
open = Abrir
delete = Eliminar
edit = Editar
create = Crear
update = Actualizar
refresh = Actualizar

## Actions
save = Guardar
cancel-button = Cancelar
ok = Aceptar
yes = Sí
no = No
continue = Continuar
back = Volver
next = Siguiente
finish = Finalizar

## Categorías de Navegación
about = Acerca de
account = Cuenta
advanced = Avanzado
danmu = Danmu
developer = Desarrollador
languages = Idiomas
privacy-security = Privacidad y Seguridad
streaming = Transmisión

## Acciones Comunes
cancel = Cancelar
create = Crear
delete = Eliminar
refresh = Actualizar
save-button = Guardar
sign-in = Iniciar Sesión
update = Actualizar
log-out = Cerrar sesión

## Configuración de Cuenta
account-greeting = Hola, @{ $handle }.
edit-profile-bluesky = Editar perfil (en Bluesky)
change-name-color = Cambiar color de nombre

## Gestión de Claves
key-management = Gestión de Claves
key-manager = Gestor de Claves
manage-keys = Gestionar Claves
your-stream-pubkeys = Tus Claves Públicas de Transmisión
no-keys = No hay claves configuradas
pubkey-description = Las claves públicas se emparejan con claves de transmisión (usadas en software de transmitiendo) para firmar y verificar tu transmisión

keys-count = { $count ->
    [one] { $count } clave
    [many] { $count } claves
   *[other] { $count } claves
}

## Gestión de Webhooks
webhooks = Webhooks
webhook-integrations = Integraciones de Webhook
webhook-integrations-description = Conecta servicios externos para recibir actualizaciones en tiempo real sobre tus transmisiones
create-webhook = Crear Webhook
edit-webhook = Editar Webhook
delete-webhook = Eliminar Webhook
no-webhooks-yet = Aún no hay webhooks configurados
failed-load-webhooks = Error al cargar webhooks
webhook-will-no-longer-receive-events = Este webhook ya no recibirá eventos
create-first-webhook-description = Crea tu primer webhook para empezar a recibir eventos de transmisión
example-captain-hook = Capitán Garfio
webhooks-count = { $count ->
    [one] { $count } webhook
   *[other] { $count } webhooks
}

## Eventos de Webhook
activates-on = Activa en:
events-livestream = Eventos de Transmisión en Directo
events-chat = Eventos de Chat
untitled-webhook = Webhook Sin Título
inactive = Inactivo

## Grabación de Depuración
debug-recording = Grabación de Depuración

## Configuración de Danmu
danmu-enabled = Habilitar Danmu
danmu-enabled-description = Muestra mensajes de chat en vivo como comentarios flotantes en tu pantalla
danmu-opacity = Opacidad
danmu-speed = Velocidad
danmu-lane-count = Número de Carriles
danmu-max-messages = Mensajes Máximos

## General
app-version-description = Información de la versión actual
confirm-delete = ¿Estás seguro de que quieres eliminar esto?
action-cannot-be-undone = Esta acción no se puede deshacer
name-optional = Nombre (opcional)
deleting = Eliminando...
saving = Guardando...
go-to-dashboard = Ir al Panel
need-setup-live-dashboard = Primero necesitas configurar una transmisión en directo en el panel
no-languages-found = No se encontraron idiomas
backup-save = Guardar ajustes de copia de seguridad
backup-saving = Guardando ajustes de copia de seguridad...
backup-secret-key-set-placeholder = (Contraseña ya establecida)
backup-error-invalid-endpoint = Debe ser un nombre de dominio válido
backup-error-invalid-bucket = Debe contener solo letras minúsculas, números, puntos y guiones
backup-error-invalid-segment-duration = Debe estar entre 1 y 60 segundos
backup-error-load-failed = Error al cargar los ajustes de almacenamiento
backup-error-update-failed = Error al actualizar el estado de la copia de seguridad
backup-error-save-failed = Error al guardar los ajustes de almacenamiento
backup-error-missing-secret = No se pueden actualizar los ajustes de S3 sin la clave secreta. Por favor, vuelva a introducirla.
backup-segment-duration-placeholder = 6

## Backup Settings
backup = Copia de seguridad
backup-enabled = S3 Backup
backup-enabled-description = Respaldar automáticamente las grabaciones en almacenamiento compatible con S3
backup-connection-url = URL de conexión
backup-connection-url-placeholder = s3+https://ACCESS_KEY:SECRET_KEY@s3.example.com/bucket
backup-endpoint = Punto de conexión
backup-endpoint-placeholder = s3.example.com
backup-bucket = Contenedor
backup-bucket-placeholder = my-backup-bucket
backup-access-key = Clave de acceso
backup-access-key-placeholder = AKIAIOSFODNN7EXAMPLE
backup-secret-key = Clave secreta
backup-secret-key-placeholder = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
show-password-in-url = Mostrar contraseña en la URL
show-password-in-url-description = Mostrar la clave secreta en la URL de conexión (si se introduce)

## Recommendations
recommendations-to-others = Recomendaciones a otros
recommendations-description = Comparte hasta 8 streamers que recomiendas a tus espectadores
no-recommendations-yet = No hay recomendaciones configuradas todavía

## Multistreaming
multistream = Multi-transmisión
multistream-targets = Objetivos de multi-transmisión
multistream-description = Envía automáticamente tus transmisiones en directo de Streamplace a otros servicios de streaming como Twitch o YouTube.
create-multistream-target = Crear objetivo de multi-transmisión
untitled-multistream-target = Objetivo sin título
failed-load-multistream-targets = Error al cargar los objetivos de multi-transmisión. Por favor, inténtalo de nuevo.
failed-toggle-multistream-target = Error al cambiar el estado del objetivo de multi-transmisión. Por favor, inténtalo de nuevo.
no-multistream-targets-yet = ¡Aún no hay objetivos!
multistream-targets-count = { $count ->
    [one] { $count } objetivo
    [many] { $count } objetivos
   *[other] { $count } objetivos
}
multistream-delete-target-confirmation = ¿Estás seguro de que quieres eliminar "{ $target }"?
this-action-cannot-be-undone = Esta acción no se puede deshacer.
rtmp-target-name = Objetivo RTMP
rtmp-target-url = URL de RTMP
rtmp-target-name-placeholder = Mi objetivo de multi-transmisión
multistream-create-target = Crear objetivo
multistream-edit-target = Editar objetivo
created = creado
status = estado

## Branding Administration
branding = Branding
branding-admin = Administración de branding
branding-admin-description = Personaliza tu instancia de Streamplace. Ten en cuenta que la configuración puede tardar varias horas en propagarse.
branding-login-required = Por favor, inicia sesión para gestionar el branding
branding-configuration = Configuración
branding-text-settings = Configuración de texto
branding-colors = Colores
branding-legal-links = Vínculos legales
branding-images = Imágenes

## Branding Fields
branding-broadcaster-did = DID del radiodifusor
branding-broadcaster-did-description = Dejar vacío para usar el valor predeterminado del servidor
branding-site-title = Título del sitio
branding-site-title-placeholder = Introduce un nuevo título de sitio
branding-site-description = Descripción del sitio
branding-site-description-placeholder = Introduce la descripción del sitio
branding-default-streamer = Streamer predeterminado
branding-default-streamer-none = Ninguno
branding-default-streamer-placeholder = did:plc:...
branding-clear-default-streamer = Quitar streamer predeterminado
branding-primary-color = Color principal
branding-primary-color-placeholder = #6366f1
branding-accent-color = Color de acento
branding-accent-color-placeholder = #8b5cf6
branding-main-logo = Logo principal
branding-main-logo-description = SVG, PNG o JPEG (máx. 500KB)
branding-favicon = Favicon
branding-favicon-description = SVG, PNG o ICO (máx. 100KB)
branding-sidebar-bg = Imagen de fondo de la barra lateral
branding-sidebar-bg-description = SVG, PNG o JPEG (máx. 500kb) - aparece alineada en la parte inferior de la barra lateral, a todo el ancho. Sube una imagen con opacidad para mejores resultados, ya que actualmente no hay una opción de opacidad separada.
branding-current = Actual: { $value }
branding-dimensions = { $height } x { $width }

## Branding Actions
branding-upload-logo = Subir logo
branding-delete-logo = Eliminar logo
branding-upload-favicon = Subir favicon
branding-delete-favicon = Eliminar favicon
branding-upload-background = Subir imagen de fondo
branding-delete-background = Eliminar imagen de fondo
branding-web-only = La subida de imágenes solo está disponible en la web.

## Branding Legal Links
refresh-branding = Actualizar recursos de branding
branding-add-legal-link = Agregar vínculo legal
branding-edit-legal-link = Editar vínculo legal
branding-legal-link-text-placeholder = Texto del vínculo (ej., Política de Privacidad)
branding-legal-link-url-placeholder = URL (ej., https://ejemplo.com/privacidad)
add = Agregar
active = Activo
optional = opcional

## Branding Toast Messages
branding-not-authenticated = Por favor, inicia sesión primero
branding-empty-value = Por favor, introduce un valor
branding-update-success = { $key } actualizado correctamente
branding-upload-success = { $key } subido correctamente
branding-delete-success = { $key } eliminado correctamente
branding-upload-failed = Error al subir
branding-delete-failed = Error al eliminar
branding-not-available = La subida de archivos solo está disponible en la web

## Navigation Categories (About Page)
node-legal-documents = Documentos específicos del radiodifusor

## Badges
badges = Insignias
badges-cosmetic-section = Insignias cosméticas
badges-empty-state = Aún no te han otorgado ninguna insignia.
badges-failed-load = Error al cargar las insignias
badges-failed-update = Error al actualizar la selección de insignias
badges-issued-by = Otorgada por { $issuer }
badges-streamer-section = Insignias del streamer

## Issue Badges
issue-badges = Emitir insignias
issue-badges-back-to-definitions = Definiciones de insignias
issue-badges-badge-name = Nombre de la insignia
issue-badges-badge-name-placeholder = ej. VIP, Colaborador
issue-badges-badge-type = Tipo de insignia
issue-badges-choose-image = Elegir imagen
issue-badges-create-definition = Crear definición de insignia
issue-badges-create-definition-description = Define un nuevo tipo de insignia que se puede otorgar a los espectadores
issue-badges-create-definition-subtitle = Define un nuevo tipo de insignia
issue-badges-definition-created = Definición de insignia creada
issue-badges-description-optional = Descripción (opcional)
issue-badges-description-placeholder = Descripción opcional para esta insignia
issue-badges-failed-create = Error al crear la definición de insignia
issue-badges-failed-issue = Error al emitir la insignia
issue-badges-failed-load = Error al cargar las definiciones de insignias
issue-badges-image-optional = Imagen (opcional)
issue-badges-image-too-large = La imagen debe ser menor de 256KB
issue-badges-image-web-only = La subida de imágenes solo está disponible en la web
issue-badges-issue-badge = Emitir insignia
issue-badges-issue-badge-description = Emite la insignia "{ $name }" a un espectador mediante su DID
issue-badges-issued = Insignia emitida
issue-badges-issued-to = Emitida a { $did }
issue-badges-manage-description = Crea definiciones de insignias y emítelas a los espectadores
issue-badges-recipient-did = DID del destinatario
issue-badges-recipient-did-placeholder = did:plc:...
issue-badges-tap-to-issue = Toca para emitir
issue-badges-your-definitions = Tus definiciones de insignias

bio = Bio
bio-preview = Vista previa
confirm-import = Confirmar importación
confirm-import-title = Confirmar importación
description-placeholder = Escribe algo sobre ti...
edit-description = Editar descripción
import = Importar
import-anyway = Importar de todas formas
import-from-leaflet = Importar desde Leaflet
import-from-leaflet-desc = Pega una URL pública de leaflet.pub o AT uri compatible
save-bio = Guardar Bio
select-panels = Seleccionar paneles
social-links = Redes sociales
