SH Tunnel Manager — 実装タスク（Go言語 + tview 実装版／XDG準拠・ポート列分割）
 
 ## 1. プロジェクト初期化
 - [ ] Goモジュール初期化（`go mod init github.com/yourname/tunnelman`）
 - [ ] ディレクトリ構成作成  
   - `cmd/tunnelman` — エントリポイント  
   - `internal/tui` — TUI関連コード  
   - `internal/core` — ビジネスロジック  
   - `internal/store` — 設定・PIDファイル管理
 - [ ] `.gitignore` 整備（ビルド成果物、`~/.config/tunnelman/` や `~/.local/state/tunnelman/` は除外）
 
 ---
 
 ## 2. データモデルとストア
 - [ ] 型定義（`internal/core/types.go`）
   - **Tunnel**: `ID, Name, Host, LocalPort, RemotePort, Mode, Profile`
   - **PidEntry**: `PID, Started`
 - [ ] 設定読み書き（`internal/store/config.go`）
   - **LoadConfig/SaveConfig**（JSON、必須項目バリデーション）
   - 保存先: `$XDG_CONFIG_HOME/tunnelman/config.json`（未設定時は`~/.config/tunnelman/config.json`）
 - [ ] PID読み書き（`internal/store/pids.go`）
   - **LoadPids/SavePids**, PIDの生存確認・クリーンアップ
   - 保存先: `$XDG_STATE_HOME/tunnelman/pids.json`（未設定時は`~/.local/state/tunnelman/pids.json`）
 
 ---
 
 ## 3. プロセス管理（ssh）
 - [ ] **Connect(t Tunnel) (PidEntry, error)**  
   - `exec.Command("ssh", ...)` を `-N` でspawn  
   - modeが`forward`なら`-L local:localhost:remote`、`reverse`なら`-R remote:localhost:local`
 - [ ] **Disconnect(id string, pid int) error**  
   - `FindProcess` → `Signal(SIGTERM)`、失敗時`SIGKILL`
 - [ ] spawn時のstderr/stdoutの扱い（`--debug`時のみ表示）
 
 ---
 
 ## 4. TUI構築（tview）
 - [ ] アプリ初期化、レイアウト（ヘッダー／テーブル／フッター）
 - [ ] メインテーブル（`tview.Table`）  
   - 列: 状態, 名前, ホスト名, Local Port, Remote Port, モード, 開始時刻  
   - 状態: 接続中=緑●, 未接続=灰○, 失敗=赤×  
   - モード: forward=青→, reverse=オレンジ←
 - [ ] ヘッダー: プロファイル名＋接続数＋主要キー表示
 - [ ] フッター: 動的ヘルプ（通常／検索／モーダル時で切替）
 - [ ] 入力捕捉（`SetInputCapture`）
 
 ---
 
 ## 5. キーバインド実装
 - [ ] ↑/↓: カーソル移動
 - [ ] `u`: 選択行 接続（spawn → `pids.json` 反映 → 再描画）
 - [ ] `d`: 選択行 切断（Kill → `pids.json` 反映 → 再描画）
 - [ ] `A`: 現在プロファイル 一斉接続
 - [ ] `X`: 現在プロファイル 一斉切断
 - [ ] `g`: プロファイル順送り切替（描画更新）
 - [ ] `f`: モード切替（forward/reverse をトグルし `config.json` 保存）
 - [ ] `c`: 新規追加フォーム（保存で `config.json` 追記）
 - [ ] `/`: 検索入力開始（フィルタ適用、Escで解除）
 - [ ] `r`: 削除モーダル表示 → 確定時に削除処理（下記6）
 
 ---
 
 ## 6. 削除処理（r）
 - [ ] モーダル表示文言: 「‘<name>’ を削除しますか？接続中なら切断してから削除します。」
 - [ ] 確定時の処理順
   1. 接続中判定 → Kill（エラー時は中断・通知）
   2. `pids.json` から該当ID削除 → 保存
   3. `config.json` から該当ID削除 → 保存
   4. UI更新（カーソルは次行へ）
 - [ ] キャンセルでモーダルを閉じるのみ
 - [ ] 書き込み失敗時はロールバック
 
 ---
 
 ## 7. 検索モード
 - [ ] `tview.InputField` で検索文字列入力
 - [ ] 名前・ホスト名・ポート番号で部分一致フィルタ
 - [ ] フィルタ結果をテーブルに反映
 
 ---
 
 ## 8. プロファイル管理
 - [ ] 設定構造体に `Profile` フィールド
 - [ ] 現在のプロファイルに応じてテーブル表示を切替
 - [ ] `--auto <profile>` 指定時は起動時に一括接続
 
 ---
 
 ## 9. 起動時自動接続
 - [ ] CLI引数解析（`flag` パッケージ）
 - [ ] 指定プロファイルの未接続トンネルをspawn
 - [ ] 失敗はログ化しつつ継続
 
 ---
 
 ## 10. ログとデバッグ
 - [ ] デフォルトはstderrにinfo/warn/errorを出力
 - [ ] `--debug` 指定時はspawnコマンド全文とstderr/stdoutを表示
 
 ---
 
 ## 11. ビルドと配布
 - [ ] `go build -o tunnelman ./cmd/tunnelman`
 - [ ] クロスコンパイル設定（`GOOS`, `GOARCH`）
 - [ ] README更新（インストール方法、使用例、キー操作）
 
 ---
 
 ## 12. 任意の拡張
 - [ ] 接続ログ表示（成功／失敗／PID）
 - [ ] ポート競合検出と警告表示
 - [ ] プロファイル選択モーダル
 - [ ] 削除前の自動バックアップ（config.json.bak）
 
