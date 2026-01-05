# Common UI Translations - Portuguese (Brazil)

## General UI
loading = Carregando...
error = Erro
cancel = Cancelar
confirm = Confirmar
close = Fechar
open = Abrir
ok = OK
yes = Sim
no = Não
continue = Continuar
back = Voltar
next = Próximo
finish = Concluir

## Actions
save = Salvar
delete = Excluir
edit = Editar
create = Criar
update = Atualizar
refresh = Atualizar

## Status Messages
success = Sucesso
warning = Aviso
info = Informação

## Input Placeholders
search-placeholder = Pesquisar...
message-input = Digite sua mensagem...

## Authentication & Access
please-log-in-to-access-this-page = Por favor, faça login para acessar esta página
go-to-settings = Ir para Configurações
go-back = Voltar

## Demo and Testing
welcome-user = Bem-vindo, { $username }!
notification-count = { $count ->
    [0] Nenhuma notificação
    [1] Uma notificação
   *[other] { $count } notificações
}

## Offline User
user-offline = usuário offline
user-offline-message = { $source ->
    [streamer] Parece que <1>@{ $handle } está offline</1>, mas eles recomendam assistir:
   *[default] Parece que <1>@{ $handle } está offline</1>, mas recomendamos assistir:
}
user-offline-no-recommendations = 
  Parece que <1>@{ $handle } está offline</1> agora.
  Volte mais tarde.
streaming-title = transmitindo { $title }
viewer-count = { $count ->
    [0] 0 espectadores
    [1] 1 espectador
   *[other] { $count } espectadores
}
