# Changelog

## 0.0.2

Faster start, live displays over WebRTC, and closer to the web's look.

- Opens on the last chats and the latest chat from a cache while it connects.
- Displays stream live over WebRTC in the system's web view, with pointer and keys; without one they show frames as before.
- Port windows show their page inside the window.
- Forked chats sit under the chat they came from; untitled chats show their latest message.
- Video tiles show their first frame and length; animated Space avatars play.
- The custom theme's web fonts load, such as Maple Mono from the Home Space's theme.
- The chat menu, labels and statistics open aligned to their button; statistics show in sections like the web's.
- ⌘K mixes Spaces and chats as the web does; hidden models stay out of the model picker.
- Fixed: unfolding a reply's steps in the demo crashed.
- Install on Windows from PowerShell: `irm https://github.com/markbang/cohub-mygo/releases/latest/download/install.ps1 | iex`.

## 0.0.1

First release: a Cohub desktop client drawn by MyGo on the GPU, without a WebView.

- Chats, Spaces, Space settings, commerce and wallet, activity, saves, scheduled prompts, tasks, Apps, files, boards, ports, displays.
- Settings: general, activity, referrals, billing (balance, plans, redeem), user rules, channels.
- Composer with attachments, voice, harness and model pickers, "/" prompts and skills, "@" Space mentions.
- Realtime updates, notifications, presence and danmaku, ⌘K palette, compact layout.
- Themes and copy taken from the web app, including the Home Space's custom theme.
