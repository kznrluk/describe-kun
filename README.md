# describe-kun

`describe-kun` は、ウェブページのコンテンツを取得し、大規模言語モデル（LLM）を使用して要約または質問応答を行うツールです。

## Slack Bot 機能

### 概要

Slack Botとして動作する `describe-kun` は、Botがメンションされたメッセージやスレッド内のメッセージに含まれるURLを自動的に抽出し、その内容を要約してスレッドに返信します。これにより、Slack上で共有されたリンクの内容を素早く把握することができます。

### ユースケース

-   共有されたニュース記事やブログ投稿の要点を把握する。
-   技術ドキュメントやレポートの概要をチームで共有する。
-   会議の議事録URLを共有し、その要約を即座に確認する。

### 実行方法

1.  **ビルド:**
    ```bash
    go build -o describe-kun-slack cmd/describe-kun-slack/main.go
    ```
2.  **環境変数の設定:**
    以下の環境変数を設定してください。
    *   `OPENAI_API_KEY`: OpenAI APIキー。
    *   `SLACK_BOT_TOKEN`: Slack Botのトークン（`xoxb-` で始まるもの）。
    *   `SLACK_SIGNING_SECRET`: Slack AppのSigning Secret。
    *   `PORT` (オプション): Botサーバーがリッスンするポート番号（デフォルト: `8080`）。
3.  **実行:**
    ```bash
    ./describe-kun-slack
    ```
    サーバーが起動し、指定されたポートでSlackからのイベントを待ち受けます。

### Slack App の設定

1.  **Slack Appの作成:** Slack Appを作成します ([https://api.slack.com/apps](https://api.slack.com/apps))。
2.  **Permissions:**
    *   "OAuth & Permissions" > "Scopes" > "Bot Token Scopes" に以下の権限を追加します:
        *   `app_mentions:read`: Botへのメンションを読み取るため。
        *   `chat:write`: メッセージを投稿するため。
        *   `channels:history` / `groups:history` / `im:history` / `mpim:history`: (オプション) メンションされたチャンネル/DMの履歴からURLを含むメッセージを取得する場合に必要になる可能性があります（現在の実装ではメンション時のテキストのみ解析）。
3.  **Event Subscriptions:**
    *   "Event Subscriptions" を有効にします。
    *   **Request URL:** `describe-kun-slack` を実行しているサーバーのURL（例: `http://your-server-address:8080/slack/events`）を入力します。サーバーが起動している状態で入力すると、URL検証が行われます。
    *   **Subscribe to bot events:** `app_mention` イベントを購読します。
4.  **Appのインストール:** 作成したAppをワークスペースにインストールします。

### 注意点

-   `describe-kun-slack` サーバーは、Slack APIからのリクエストを受け付けるために、外部からアクセス可能なネットワーク上にデプロイする必要があります（例: ngrok、クラウドサーバーなど）。
-   URLの抽出はシンプルな正規表現で行っています。複雑な形式のURLは抽出できない場合があります。
-   ページの取得やLLMによる要約には時間がかかることがあります。Slackの3秒タイムアウトルールに対応するため、イベント受信後すぐに `200 OK` を返し、実際の処理はバックグラウンドで行い、結果を非同期で投稿します。

## コマンドラインツール (オリジナル)

（コマンドラインツールの説明が必要な場合はここに追加）

```
./describe-kun --url <URL> [--prompt <質問>] [--timeout <タイムアウト秒>]
```
