# Common UI Translations - Spanish (Spain)

## General UI
loading = Cargando...
error = Error
cancel = Cancelar
confirm = Confirmar
close = Cerrar
open = Abrir
ok = OK
yes = Sí
no = No
continue = Continuar
back = Atrás
next = Siguiente
finish = Finalizar

## Actions
save = Guardar
delete = Eliminar
edit = Editar
create = Crear
update = Actualizar
refresh = Actualizar

## Status Messages
success = Éxito
warning = Advertencia
info = Información

## Input Placeholders
search-placeholder = Buscar...
message-input = Escribe tu mensaje...

## Authentication & Access
please-log-in-to-access-this-page = Por favor, inicia sesión para acceder a esta página
go-to-settings = Ir a Configuración
go-back = Volver

## Demo and Testing
welcome-user = ¡Bienvenido, { $username }!
notification-count = { $count ->
    [0] Sin notificaciones
    [1] Una notificación
   *[other] { $count } notificaciones
}

## Offline User
user-offline = usuario desconectado
user-offline-message = { $source ->
    [streamer] Parece que <1>@{ $handle } está desconectado</1>, pero ellos recomiendan ver:
   *[default] Parece que <1>@{ $handle } está desconectado</1>, pero te recomendamos ver:
}
user-offline-no-recommendations = 
  Parece que <1>@{ $handle } está desconectado</1> ahora mismo.
  Vuelve más tarde.
streaming-title = transmitiendo { $title }
viewer-count = { $count ->
    [0] 0 espectadores
    [1] 1 espectador
   *[other] { $count } espectadores
}

## PDS Host Selector
pds-selector-title = ¿Nuevo en Atmosphere?
pds-selector-description = Necesitarás seleccionar un PDS (Servidor de Datos Personal) para acceder a apps en Atmosphere, como Bluesky, Tangled y Spark.
pds-selector-custom-label = Otro PDS
pds-selector-custom-description = Introduce la URL de tu propio servidor PDS
pds-selector-custom-url-label = URL de PDS personalizado
pds-selector-custom-url-placeholder = https://pds.ejemplo.com
pds-selector-learn-more = Más información sobre el autoalojamiento
pds-selector-info = Cada servidor tiene sus propias políticas y estándares de fiabilidad. Tus datos ATProto viven en el servidor que elijas y puedes migrar más tarde. Nota: Streamplace tiene sus propias reglas de moderación; puedes ser expulsado de Streamplace independientemente del servidor que elijas.
pds-selector-read-policies = Lee los <tosLink>Términos de Servicio</tosLink> y la <privacyLink>Política de Privacidad</privacyLink> de { $label } antes de continuar.
pds-selector-handle-policy-checkbox = He leído y acepto la <policyLink>política de identificadores</policyLink>

## Login
login-show-live-on-bluesky = Mostrar cuando estoy en directo en Bluesky
login-show-live-on-bluesky-description = Añade el anillo rojo LIVE a tu avatar de Bluesky mientras transmites y permite que Streamplace publique anuncios por ti. Desmárcalo para iniciar sesión sin conceder ningún acceso a tu cuenta de Bluesky.

## Stream notifications
teleporting-in = Teletransportando en
teleporting-to = Teletransportando a @{ $handle }

## Teleport dialog
teleport-dialog-title = Teletransportarse a otra persona en directo
teleport-dialog-description = Selecciona a una persona en directo para enviar a tus espectadores a su stream.
teleport-unknown-streamer = Streamer desconocido
teleport-search-label = Buscar streams en directo
teleport-search-placeholder = Buscar por identificador o título
teleport-loading-streamers = Cargando streamers en directo…
teleport-empty-streamers = No se encontraron streamers en directo.
teleport-no-matching-streamers = No hay streamers en directo que coincidan.
teleport-viewer-count = { $count ->
    [one] { $count } espectador
   *[other] { $count } espectadores
}
teleport-countdown = Cuenta atrás
teleport-seconds-range = segundos (5–300)
teleport-countdown-error = La cuenta atrás debe estar entre 5 y 300 segundos.
teleport-selection-expired = Ese streamer ya no está en directo. Elige otro.
teleport-starting = Iniciando…
teleport-start = Teletransportar
teleport-error-streamer-only = Solo quien transmite el directo actual puede iniciar una teletransportación.
teleport-error-handle-format = Introduce un identificador válido, como handle.bsky.social.
teleport-error-countdown-number = La cuenta atrás debe ser un número de segundos.
teleport-error-self = No puedes teletransportarte a ti mismo.
teleport-error-resolve-handle = No se encontró a @{ $handle }.
teleport-error-create = No se pudo iniciar la teletransportación.
