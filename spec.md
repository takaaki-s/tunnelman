SH Tunnel Manager — 完全仕様書（Go言語 + tview 実装版／XDG準拠・ポート列分割）
 
 ## 1. 概要
 複数のSSHトンネル接続をCLIベースのTUIで管理するGo製ユーティリティ。  
 `tview` により、テーブル表示・フォーム・モーダルなどのリッチなUIをターミナル上で提供。  
 ユーザー定義の設定に基づき、接続・切断・状態表示・プロファイル切替・検索・新規追加・削除を直感的に操作可能。
 
 ---
 
 ## 2. 実行方法
 
 ### インストール
 ```bash
 go install github.com/yourname/tunnelman@latest
 ```
 
 ### 実行
 ```bash
 tunnelman
 ```
 
 ### 自動接続付き起動
 ```bash
 tunnelman --auto <profile>
 ```
 
 ---
 
 ## 3. データ仕様（XDG準拠）
 
 ### 3.1 保存先ディレクトリ
 - **設定ファイル**:  
   `$XDG_CONFIG_HOME/tunnelman/config.json`  
   （未設定時は `~/.config/tunnelman/config.json`）
 - **PIDファイル**:  
   `$XDG_STATE_HOME/tunnelman/pids.json`  
   （未設定時は `~/.local/state/tunnelman/pids.json`）
 
 > Windowsの場合は `%AppData%\tunnelman\config.json` と `%LocalAppData%\tunnelman\pids.json` を使用。
 
 ---
 
 ### 3.2 config.json
 - 文字コード: UTF-8
 - 構造: 配列（各要素はトンネル設定）
 
 ```jsonc
 [
   {
     "id": "web-prod",         // 一意識別子（英数字・ハイフン、小文字）
     "name": "Web サーバー",    // 表示名（自由記述、日本語可）
     "host": "example.com",    // SSH接続先ホスト名またはIP
     "localPort": 8080,        // ローカルポート（1〜65535）
     "remotePort": 80,         // リモートポート（1〜65535）
     "mode": "forward",        // "forward" または "reverse"
     "profile": "default"      // プロファイル名（空文字不可）
   }
 ]
 ```
 
 **制約**
 - `id` は全設定で一意、変更不可（内部参照用）
 - `name` はUI表示用で変更可
 - `mode` は `"forward"` または `"reverse"`
 - ポート番号は整数かつ 1〜65535
 
 ---
 
 ### 3.3 pids.json
 - 構造: オブジェクト（キーは `config.json` の `id`）
 
 ```jsonc
 {
   "web-prod": {
     "pid": 12345,                       // SSHプロセスのPID
     "started": "2025-09-15T09:00:00Z"   // ISO 8601 UTC
   }
 }
 ```
 
 **制約**
 - `pid` は接続中のみ存在
 - 切断時はエントリごと削除
 - 起動時にPIDが存在しない場合はクリーンアップ
 
 ---
 
 ## 4. UI仕様（tview）
 
 ### 4.1 レイアウト
 1. **ヘッダー**  
    - 現在のプロファイル名  
    - 主要ショートカットキー表示
 2. **メインテーブル**（`tview.Table`）  
    - 列順固定: 状態（●/○）, 名前, ホスト名, Local Port, Remote Port, モード(→/←), 開始時刻
    - 接続中は緑、未接続は灰色
 3. **フッター**  
    - 操作ガイド（キー一覧）
 4. **フォーム画面**（`c`キー）  
    - 入力項目: 名前, ホスト, ローカルポート, リモートポート, モード, プロファイル
 5. **検索モード**（`/`キー）  
    - インクリメンタルサーチ（名前・ホスト名・ポート番号）
 6. **削除モーダル**（`r`キー）  
    - 確認文言: 「‘<name>’ を削除しますか？接続中なら切断してから削除します。」
 
 ---
 
 ### 4.2 メインテーブル表示例
 ```
  状態  名前         ホスト名           Local Port  Remote Port  モード  開始時刻
  ●     web-prod     example.com        8080        80           →      09:00
  ●     db-main      db.example.com     5432        5432         →      09:02
  ○     admin-rev    admin.example.com  2222        22           ←      
 ```
 
 ---
 
 ## 5. 操作キー一覧
 | キー | 動作 |
 |------|------|
 | ↑ / ↓ | カーソル移動 |
 | `u` | 選択トンネル接続 |
 | `d` | 選択トンネル切断 |
 | `A` | プロファイル内一斉接続 |
 | `X` | プロファイル内一斉切断 |
 | `g` | プロファイル順送り切替 |
 | `f` | モード切替（forward/reverse） |
 | `c` | 新規追加フォーム表示 |
 | `/` | 検索モード |
 | `Esc` | 検索キャンセル／モーダル閉じる |
 | `r` | 選択トンネル削除（確認モーダル） |
 | `q` | 終了（接続は維持） |
 
 ---
 
 ## 6. 処理フロー
 
 ### 6.1 起動時
 1. 設定ファイル読み込み
 2. PIDファイル読み込み＆クリーンアップ
 3. `--auto` 指定時は該当プロファイルを一括接続
 4. UI描画開始
 
 ### 6.2 接続
 1. `ssh` コマンド構築（modeに応じて -L/-R）
 2. spawn → PID取得
 3. `pids.json` に記録
 4. UI更新
 
 ### 6.3 切断
 1. PID取得
 2. Kill（SIGTERM、失敗時SIGKILL）
 3. `pids.json` から削除
 4. UI更新
 
 ### 6.4 削除
 1. 接続中なら切断処理
 2. `pids.json` から削除
 3. `config.json` から設定削除
 4. UI更新（カーソルは次行へ）
 
 ---
 
 ## 7. エラー処理ポリシー
 - ファイル書き込み失敗時は即エラー表示＆処理中断
 - PID存在しない場合は警告表示しクリーンアップ
 - 接続失敗時は赤色で「FAILED」表示
 - 削除時にKill失敗した場合は設定削除を中断
 
 ---
 
 ## 8. ログ
 - デフォルトはstderrにinfo/warn/errorを出力
 - `--debug` 指定時はspawnコマンド全文とstderr/stdoutを表示
 
 ---
 
 ## 9. 終了時
 - `q` では接続維持
 - Ctrl+C では接続維持（SIGINTハンドラでUI終了のみ）
 
 ---
 
 ## 10. 将来拡張（任意）
 - プロファイル選択モーダル
 - 削除前の自動バックアップ（config.json.bak）
 - ポート競合検出と警告
 
