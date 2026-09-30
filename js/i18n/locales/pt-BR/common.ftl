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

## PDS Host Selector
pds-selector-title = Novo no Atmosphere?
pds-selector-description = Você precisará selecionar um PDS (Servidor de Dados Pessoal) para acessar apps no Atmosphere, como Bluesky, Tangled e Spark.
pds-selector-custom-label = Outro PDS
pds-selector-custom-description = Digite a URL do seu próprio host PDS
pds-selector-custom-url-label = URL do PDS personalizado
pds-selector-custom-url-placeholder = https://pds.exemplo.com
pds-selector-learn-more = Saiba mais sobre auto-hospedagem
pds-selector-info = Cada host tem suas próprias políticas e padrões de confiabilidade. Seus dados ATProto ficam no host que você escolher e você pode migrar depois. Nota: O Streamplace tem suas próprias regras de moderação — você pode ser banido do Streamplace independentemente do host escolhido.
pds-selector-read-policies = Leia os <tosLink>Termos de Serviço</tosLink> e a <privacyLink>Política de Privacidade</privacyLink> de { $label } antes de continuar.
pds-selector-handle-policy-checkbox = Li e concordo com a <policyLink>política de identificadores</policyLink>

## Login
login-show-live-on-bluesky = Mostrar quando estou ao vivo no Bluesky
login-show-live-on-bluesky-description = Adiciona o anel vermelho LIVE ao seu avatar do Bluesky enquanto você transmite e permite que o Streamplace publique anúncios por você. Desmarque para entrar sem conceder nenhum acesso à sua conta do Bluesky.

## Stream notifications
teleporting-in = Teletransportando em
teleporting-to = Teletransportando para @{ $handle }

## Teleport dialog
teleport-dialog-title = Teletransportar para outro streamer ao vivo
teleport-dialog-description = Selecione um streamer para enviar seus espectadores ao canal dele.
teleport-unknown-streamer = Streamer desconhecido
teleport-search-label = Buscar transmissões ao vivo
teleport-search-placeholder = Buscar por @ ou título
teleport-loading-streamers = Carregando streamers ao vivo…
teleport-empty-streamers = Nenhum streamer ao vivo encontrado.
teleport-no-matching-streamers = Nenhum streamer ao vivo corresponde à busca.
teleport-viewer-count = { $count ->
    [one] { $count } espectador
   *[other] { $count } espectadores
}
teleport-countdown = Contagem regressiva
teleport-seconds-range = segundos (5–300)
teleport-countdown-error = A contagem deve ficar entre 5 e 300 segundos.
teleport-selection-expired = Esse streamer não está mais ao vivo. Escolha outro.
teleport-starting = Iniciando…
teleport-start = Teletransportar
teleport-error-streamer-only = Apenas o streamer da transmissão atual pode iniciar um teletransporte.
teleport-error-handle-format = Digite um identificador válido, como handle.bsky.social.
teleport-error-countdown-number = A contagem deve ser um número de segundos.
teleport-error-self = Você não pode se teletransportar para si mesmo.
teleport-error-resolve-handle = Não foi possível encontrar @{ $handle }.
teleport-error-create = Não foi possível iniciar o teletransporte.
