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
