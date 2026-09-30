# Common UI Translations - Chinese (Simplified)

## General UI
loading = 正在加载...
error = 错误
cancel = 取消
confirm = 确认
close = 关闭
open = 开启
ok = 确定
yes = 是
no = 否
continue = 继续
back = 返回
next = 下一步
finish = 完成

## Actions
save = 保存
delete = 删除
edit = 编辑
create = 创建
update = 更新
refresh = 刷新

## Status Messages
success = 成功
warning = 警告
info = 信息

## Input Placeholders
search-placeholder = 搜索...
message-input = 输入您的消息...

## Authentication & Access
please-log-in-to-access-this-page = 请登录以访问此页面
go-to-settings = 前往设置
go-back = 返回

## Demo and Testing
welcome-user = 欢迎，{ $username }！
notification-count = { $count ->
    [0] 无通知
   *[other] { $count } 则通知
}

## Offline User
user-offline = 用户离线
user-offline-message = { $source ->
    [streamer] 看起来 <1>@{ $handle } 离线</1>了，但他们推荐观看：
   *[default] 看起来 <1>@{ $handle } 离线</1>了，但我们推荐观看：
}
user-offline-no-recommendations =
  看起来 <1>@{ $handle } 离线</1>了。
  请稍后再来看看。
streaming-title = 正在直播 { $title }
viewer-count = { $count } 位观众

## PDS Host Selector
pds-selector-title = 刚接触 Atmosphere 吗？
pds-selector-description = 您需要选择一个 PDS (个人数据服务器) 来访问 Atmosphere 上的应用程序，例如 Bluesky、Tangled 和 Spark。
pds-selector-custom-label = 另一个 PDS
pds-selector-custom-description = 请输入您自己的 PDS 主机网址
pds-selector-custom-url-label = 自定义 PDS 网址
pds-selector-custom-url-placeholder = https://pds.example.com
pds-selector-learn-more = 了解有关自组主机的更多信息
pds-selector-info = 每个主机都有自己的政策和可靠性标准。您的 ATProto 数据存储在您选择的主机上，您可以以后进行迁移。注意：Streamplace 有自己的审核规则——无论您选择哪个主机，都可能会被 Streamplace 禁止使用。
pds-selector-read-policies = 在继续之前，请阅读 { $label } 的<tosLink>服务条款</tosLink>和<privacyLink>隐私政策</privacyLink>。
pds-selector-handle-policy-checkbox = 我已阅读并同意<policyLink>处理政策</policyLink>

## Login
login-show-live-on-bluesky = 在 Bluesky 上显示我的直播状态
login-show-live-on-bluesky-description = 直播时为您的 Bluesky 头像加上红色 LIVE 圆环，并允许 Streamplace 代您发布直播公告。取消勾选即可在不授予任何 Bluesky 账号访问权限的情况下登录。

## Stream notifications
teleporting-in = 传送倒计时
teleporting-to = 正在传送至 @{ $handle }

## Teleport dialog
teleport-dialog-title = 传送至其他直播主播
teleport-dialog-description = 选择一位主播，将观众传送到对方的直播间。
teleport-unknown-streamer = 未知主播
teleport-search-label = 搜索直播
teleport-search-placeholder = 按账号或标题搜索
teleport-loading-streamers = 正在加载直播主播…
teleport-empty-streamers = 未找到直播主播。
teleport-no-matching-streamers = 没有符合条件的直播主播。
teleport-viewer-count = { $count } 位观众
teleport-countdown = 倒计时
teleport-seconds-range = 秒（5–300）
teleport-countdown-error = 倒计时必须为 5 到 300 秒。
teleport-selection-expired = 该主播已不在直播。请选择其他主播。
teleport-starting = 正在启动…
teleport-start = 开始传送
teleport-error-streamer-only = 只有当前直播的主播可以发起传送。
teleport-error-handle-format = 请输入有效账号，例如 handle.bsky.social。
teleport-error-countdown-number = 倒计时必须是秒数。
teleport-error-self = 无法传送到自己的直播间。
teleport-error-resolve-handle = 找不到 @{ $handle }。
teleport-error-create = 无法启动传送。
teleport-error-generic = 无法启动传送：{ $message }
