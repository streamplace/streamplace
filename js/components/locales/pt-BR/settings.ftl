# Traduções da Página de Configurações - Português (Brasil)

## Versão do Aplicativo
app-version = Streamplace v{ $version }
download-new-update = Baixar Nova Atualização
check-for-updates = Verificar Atualizações

bundled-runtype = Empacotado
ota-runtype = Over-the-Air (OTA)
recovery-runtype = Modo de Recuperação

modal-latest-version = Você está usando a versão mais recente.
modal-no-update-available = Você está na versão mais recente do Streamplace, eba!
modal-update-available-title = Atualização Disponível
modal-update-available-description = Uma nova versão do Streamplace está pronta para download
modal-update-failed = A verificação de atualizações falhou. Você pode precisar atualizar o aplicativo através do { $store }.
modal-update-failed-title = Atualização Falhou
modal-update-failed-description = A verificação de atualizações falhou. Você pode precisar atualizar o aplicativo através do { $store }.
button-reload-app-on-update = Aplicar Atualização (o aplicativo será recarregado)

## Configurações de Nó Personalizado
use-custom-node = Usar Nó Personalizado
default-url = Padrão: { $url }
enter-custom-node-url = Digite a URL do nó personalizado
save-button = SALVAR

## Configurações de Idioma
language-selection = Idioma
language-selection-description = Escolha seu idioma preferido
help-translate = Ajude-nos a traduzir o Streamplace
help-translate-description = Estamos procurando voluntários para ajudar a traduzir o Streamplace para mais idiomas. Se você estiver interessado, entre em contato conosco no Discord ou GitHub!
currently-translating = As traduções estão a caminho
currently-translating-description = Algumas partes do aplicativo podem parecer incompletas. Obrigado pela sua paciência!

## Gravação de Depuração
debug-recording-title = Permitir que { $host } grave sua transmissão ao vivo para depuração e melhoria do serviço
debug-recording-description = Opcional
input-search-languages = Pesquisar idiomas...

## Gerenciamento de Chaves
manage-keys = Gerenciar Chaves

## Interface Geral
settings-title = Configurações
loading = Carregando...
error = Erro
cancel = Cancelar
confirm = Confirmar

## Demonstração e Testes
welcome-user = Bem-vindo, { $username }!
notification-count = { $count ->
    [0] Nenhuma notificação
    [1] Uma notificação
   *[other] { $count } notificações
}
search-placeholder = Pesquisar...
message-input = Digite sua mensagem...

## Mensagens de Status
success = Sucesso
warning = Aviso
info = Informação
close = Fechar
open = Abrir
delete = Excluir
edit = Editar
create = Criar
update = Atualizar
refresh = Atualizar

## Ações
save = Salvar
cancel-button = Cancelar
ok = OK
yes = Sim
no = Não
continue = Continuar
back = Voltar
next = Próximo
finish = Finalizar

## Categorias de Navegação
about = Sobre
account = Conta
advanced = Avançado
danmu = Danmu
developer = Desenvolvedor
languages = Idiomas
privacy-security = Privacidade e Segurança
streaming = Transmissão

## Ações Comuns
cancel = Cancelar
create = Criar
delete = Excluir
refresh = Atualizar
save-button = Salvar
sign-in = Entrar
update = Atualizar
log-out = Sair

## Configurações da Conta
account-greeting = Olá, @{ $handle }.
edit-profile-bluesky = Editar perfil (no Bluesky)
change-name-color = Mudar cor do nome

## Gerenciamento de Chaves
key-management = Gerenciamento de Chaves
key-manager = Gerenciador de Chaves
manage-keys = Gerenciar Chaves
your-stream-pubkeys = Suas Chaves Públicas de Transmissão
no-keys = Nenhuma chave configurada
pubkey-description = Chaves públicas são emparelhadas com chaves de transmissão (usadas em software de transmitindo) para assinar e verificar sua transmissão

keys-count = { $count ->
    [one] { $count } chave
    [many] { $count } chaves
   *[other] { $count } chaves
}

## Gerenciamento de Webhooks
webhooks = Webhooks
webhook-integrations = Integrações de Webhook
webhook-integrations-description = Conecte serviços externos para receber atualizações em tempo real sobre suas transmissões
create-webhook = Criar Webhook
edit-webhook = Editar Webhook
delete-webhook = Excluir Webhook
no-webhooks-yet = Nenhum webhook configurado ainda
failed-load-webhooks = Falha ao carregar webhooks
webhook-will-no-longer-receive-events = Este webhook não receberá mais eventos
create-first-webhook-description = Crie seu primeiro webhook para começar a receber eventos de transmissão
example-captain-hook = Capitão Gancho
webhooks-count = { $count ->
    [one] { $count } webhook
   *[other] { $count } webhooks
}

## Eventos de Webhook
activates-on = Ativa em:
events-livestream = Eventos de Transmissão ao Vivo
events-chat = Eventos de Chat
untitled-webhook = Webhook Sem Título
inactive = Inativo

## Gravação de Depuração
debug-recording = Gravação de Depuração

## Configurações de Danmu
danmu-enabled = Ativar Danmu
danmu-enabled-description = Exibir mensagens de chat ao vivo como comentários flutuantes na sua tela
danmu-opacity = Opacidade
danmu-speed = Velocidade
danmu-lane-count = Número de Faixas
danmu-max-messages = Mensagens Máximas

## Geral
app-version-description = Informações da versão atual
confirm-delete = Tem certeza de que deseja excluir isso?
action-cannot-be-undone = Esta ação não pode ser desfeita
name-optional = Nome (opcional)
deleting = Excluindo...
saving = Salvando...
go-to-dashboard = Ir para o Painel
need-setup-live-dashboard = Você precisa configurar uma transmissão ao vivo no painel primeiro
no-languages-found = Nenhum idioma encontrado
backup-save = Salvar configurações de backup
backup-saving = Salvando configurações de backup...
backup-secret-key-set-placeholder = (Senha já definida)
backup-error-invalid-endpoint = Deve ser um nome de domínio válido
backup-error-invalid-bucket = Deve conter apenas letras minúsculas, números, pontos e hifens
backup-error-invalid-segment-duration = Deve estar entre 1 e 60 segundos
backup-error-load-failed = Falha ao carregar as configurações de armazenamento
backup-error-update-failed = Falha ao atualizar o status do backup
backup-error-save-failed = Falha ao salvar as configurações de armazenamento
backup-error-missing-secret = Não é possível atualizar as configurações do S3 sem a chave secreta. Por favor, insira-a novamente.
backup-segment-duration-placeholder = 6

## Backup Settings
backup = Backup
backup-enabled = S3 Backup
backup-enabled-description = Fazer backup automaticamente das gravações em armazenamento compatível com S3
backup-connection-url = URL de conexão
backup-connection-url-placeholder = s3+https://ACCESS_KEY:SECRET_KEY@s3.example.com/bucket
backup-endpoint = Endpoint
backup-endpoint-placeholder = s3.example.com
backup-bucket = Bucket
backup-bucket-placeholder = my-backup-bucket
backup-access-key = Chave de acesso
backup-access-key-placeholder = AKIAIOSFODNN7EXAMPLE
backup-secret-key = Chave secreta
backup-secret-key-placeholder = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
show-password-in-url = Mostrar senha na URL
show-password-in-url-description = Exibir a chave secreta na URL de conexão (se inserida)

## Recommendations
recommendations-to-others = Recomendações para outros
recommendations-description = Compartilhe até 8 streamers que você recomenda aos seus espectadores
no-recommendations-yet = Nenhuma recomendação configurada ainda

## Multistreaming
multistream = Multitransmissão
multistream-targets = Destinos de multitransmissão
multistream-description = Envie automaticamente suas transmissões ao vivo do Streamplace para outros serviços como Twitch ou YouTube.
create-multistream-target = Criar destino de multitransmissão
untitled-multistream-target = Destino sem título
failed-load-multistream-targets = Falha ao carregar os destinos de multitransmissão. Por favor, tente novamente.
failed-toggle-multistream-target = Falha ao alternar o destino de multitransmissão. Por favor, tente novamente.
no-multistream-targets-yet = Nenhum destino ainda!
multistream-targets-count = { $count ->
    [one] { $count } destino
    [many] { $count } destinos
   *[other] { $count } destinos
}
multistream-delete-target-confirmation = Tem certeza de que deseja excluir "{ $target }"?
this-action-cannot-be-undone = Esta ação não pode ser desfeita.
rtmp-target-name = Destino RTMP
rtmp-target-url = URL RTMP
rtmp-target-name-placeholder = Meu destino de multitransmissão
multistream-create-target = Criar destino
multistream-edit-target = Editar destino
created = criado
status = status

## Branding Administration
branding = Identidade visual
branding-admin = Administração de identidade visual
branding-admin-description = Personalize sua instância do Streamplace. As configurações podem levar algumas horas para se propagar.
branding-login-required = Faça login para gerenciar a identidade visual
branding-configuration = Configuração
branding-text-settings = Configurações de texto
branding-colors = Cores
branding-legal-links = Links legais
branding-images = Imagens

## Branding Fields
branding-broadcaster-did = DID do transmissor
branding-broadcaster-did-description = Deixe vazio para usar o padrão do servidor
branding-site-title = Título do site
branding-site-title-placeholder = Digite o novo título do site
branding-site-description = Descrição do site
branding-site-description-placeholder = Digite a descrição do site
branding-default-streamer = Streamer padrão
branding-default-streamer-none = Nenhum
branding-default-streamer-placeholder = did:plc:...
branding-clear-default-streamer = Limpar streamer padrão
branding-primary-color = Cor principal
branding-primary-color-placeholder = #6366f1
branding-accent-color = Cor de destaque
branding-accent-color-placeholder = #8b5cf6
branding-main-logo = Logo principal
branding-main-logo-description = SVG, PNG ou JPEG (máx. 500KB)
branding-favicon = Favicon
branding-favicon-description = SVG, PNG ou ICO (máx. 100KB)
branding-sidebar-bg = Imagem de fundo da barra lateral
branding-sidebar-bg-description = SVG, PNG ou JPEG (máx. 500kb) - aparece alinhada na parte inferior da barra lateral, largura total. Faça upload de uma imagem com opacidade para melhores resultados, pois atualmente não há uma opção de opacidade separada.
branding-current = Atual: { $value }
branding-dimensions = { $height } x { $width }

## Branding Actions
branding-upload-logo = Enviar logo
branding-delete-logo = Excluir logo
branding-upload-favicon = Enviar favicon
branding-delete-favicon = Excluir favicon
branding-upload-background = Enviar plano de fundo
branding-delete-background = Excluir plano de fundo
branding-web-only = O upload de imagens só está disponível na web.

## Branding Legal Links
refresh-branding = Atualizar recursos de identidade visual
branding-add-legal-link = Adicionar link legal
branding-edit-legal-link = Editar link legal
branding-legal-link-text-placeholder = Texto do link (ex., Política de Privacidade)
branding-legal-link-url-placeholder = URL (ex., https://exemplo.com/privacidade)
add = Adicionar
active = Ativo
optional = opcional

## Branding Toast Messages
branding-not-authenticated = Faça login primeiro
branding-empty-value = Por favor, insira um valor
branding-update-success = { $key } atualizado com sucesso
branding-upload-success = { $key } enviado com sucesso
branding-delete-success = { $key } excluído com sucesso
branding-upload-failed = Falha ao enviar
branding-delete-failed = Falha ao excluir
branding-not-available = O upload de arquivos só está disponível na web

## Navigation Categories (About Page)
node-legal-documents = Documentos específicos do transmissor

## Badges
badges = Emblemas
badges-cosmetic-section = Emblemas cosméticos
badges-empty-state = Você ainda não recebeu nenhum emblema.
badges-failed-load = Falha ao carregar emblemas
badges-failed-update = Falha ao atualizar a seleção de emblemas
badges-issued-by = Emitido por { $issuer }
badges-streamer-section = Emblemas do streamer

## Issue Badges
issue-badges = Emitir emblemas
issue-badges-back-to-definitions = Definições de emblemas
issue-badges-badge-name = Nome do emblema
issue-badges-badge-name-placeholder = ex. VIP, Apoiador
issue-badges-badge-type = Tipo de emblema
issue-badges-choose-image = Escolher imagem
issue-badges-create-definition = Criar definição de emblema
issue-badges-create-definition-description = Defina um novo tipo de emblema que pode ser emitido para espectadores
issue-badges-create-definition-subtitle = Defina um novo tipo de emblema
issue-badges-definition-created = Definição de emblema criada
issue-badges-description-optional = Descrição (opcional)
issue-badges-description-placeholder = Descrição opcional para este emblema
issue-badges-failed-create = Falha ao criar a definição de emblema
issue-badges-failed-issue = Falha ao emitir o emblema
issue-badges-failed-load = Falha ao carregar as definições de emblemas
issue-badges-image-optional = Imagem (opcional)
issue-badges-image-too-large = A imagem deve ter menos de 256KB
issue-badges-image-web-only = O upload de imagens só está disponível na web
issue-badges-issue-badge = Emitir emblema
issue-badges-issue-badge-description = Emita o emblema "{ $name }" para um espectador pelo seu DID
issue-badges-issued = Emblema emitido
issue-badges-issued-to = Emitido para { $did }
issue-badges-manage-description = Crie definições de emblemas e emita-os para espectadores
issue-badges-recipient-did = DID do destinatário
issue-badges-recipient-did-placeholder = did:plc:...
issue-badges-tap-to-issue = Toque para emitir
issue-badges-your-definitions = Suas definições de emblemas

bio = Bio
bio-preview = Prévia
confirm-import = Confirmar Importação
confirm-import-title = Confirmar Importação
description-placeholder = Escreva algo sobre você...
edit-description = Editar Descrição
import = Importar
import-anyway = Importar mesmo assim
import-from-leaflet = Importar do Leaflet
import-from-leaflet-desc = Cole um URL público do leaflet.pub ou AT uri compatível
save-bio = Salvar Bio
select-panels = Selecionar Painéis
social-links = Links Sociais
