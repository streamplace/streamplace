# Common UI Translations - Chinese (Traditional)

## General UI
loading = 載入中...
error = 錯誤
cancel = 取消
confirm = 確認
close = 關閉
open = 開啟
ok = 確定
yes = 是
no = 否
continue = 繼續
back = 返回
next = 下一步
finish = 完成

## Actions
save = 儲存
delete = 刪除
edit = 編輯
create = 建立
update = 更新
refresh = 重新整理

## Status Messages
success = 成功
warning = 警告
info = 資訊

## Input Placeholders
search-placeholder = 搜尋...
message-input = 輸入您的訊息...

## Authentication & Access
please-log-in-to-access-this-page = 請登入以存取此頁面
go-to-settings = 前往設定
go-back = 返回

## Demo and Testing
welcome-user = 歡迎，{ $username }！
notification-count = { $count ->
    [0] 無通知
    [1] 一則通知
   *[other] { $count } 則通知
}

## Offline User
user-offline = 使用者離線
user-offline-message = { $source ->
    [streamer] 看起來 <1>@{ $handle } 離線</1> 了，但他們推薦觀看：
   *[default] 看起來 <1>@{ $handle } 離線</1> 了，但我們推薦觀看：
}
user-offline-no-recommendations = 
  看起來 <1>@{ $handle } 離線</1> 了。
  請稍後再來看看。
streaming-title = 正在直播 { $title }
viewer-count = { $count ->
    [0] 0 位觀眾
    [1] 1 位觀眾
   *[other] { $count } 位觀眾
}
