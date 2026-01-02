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
