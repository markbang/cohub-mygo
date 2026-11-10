# Changelog

## 0.0.3

Updates itself, streams replies to the end, and works more like the web.

- The app checks for updates and offers to install them; **Check for Updates…** is in the account menu, and on macOS in the app menu.
- Replies keep streaming after tool calls, through every model call of a turn.
- Messages sent while a turn runs wait above the composer, to steer now or cancel.
- Drag the edges of the sidebar, the right panel and a window to resize them; the widths are kept.
- Hovering the turn rail opens the turn list with search; its track scrolls the chat, and older turns load as you scroll up.
- Tool calls open into their input and output; the tokens and time under a reply open its statistics.
- File tree rows show insert, rename, upload and more on hover; archived chats can be restored above the composer.
- Tooltips on message times, models, authors and steps, as on the web.
- In the compact layout, a button over the chat jumps to the latest and opens the turn list.
- MyGo's own texts, as the menus', follow the language chosen in the settings.

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
