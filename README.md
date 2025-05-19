# Wg-Knot

Wg-Knot is a relay program that lets two WireGuard peers behind NAT exchange traffic **without decrypting it en route**. Because it speaks the native WireGuard protocol, no changes are required on the peers themselves.

## Usage

```bash
# Build
go build -o wg-knot .

# Create a config file
cp setting.conf.example setting.conf

# Run
./wg-knot
```

## Configuration

### Configuration file

Start with `setting.conf.example` and adjust it to your needs.

### Environment variables

| Variable | Description | Default |
|-------------------------|-------------------------------------------|------------------|
| `WG_KNOT_CONFIG_FILE`   | Path to the configuration file            | `./setting.conf` |
| `WG_KNOT_LISTEN_ADDRESS`| IP address to listen on                   | `0.0.0.0`        |
| `WG_KNOT_PORT`          | UDP port to listen on                     | `52820`          |
| `WG_KNOT_LOG_LEVEL`     | Log level (`debug`, `info`, `warn`, etc.) | `info`           |

### Command-line flags

| Flag          | Description                         |
|---------------|-------------------------------------|
| `-configfile` | Path to the configuration file      |
| `-listen`     | IP address to listen on             |
| `-port`       | UDP port to listen on               |
| `-loglevel`   | Log level                           |

## Example

1. **Start wg-knot**

   ```bash
   ./wg-knot
   ```

   Minimal `setting.conf`:

   ```
   [server]
   port = <port where Wg-Knot listens>

   # Allowed public-key pairs
   [[keypairs]]
   key1 = "<public key of peer A>"
   key2 = "<public key of peer B>"
   ```

2. **Configure each peer and connect**

   **Peer A**

   ```ini
   [Interface]
   PrivateKey = <peer A private key>
   Address    = 10.0.0.1/32

   [Peer]
   PublicKey  = <peer B public key>
   Endpoint   = <Wg-Knot server IP>:<Wg-Knot port>
   AllowedIPs = 10.0.0.2/32
   PersistentKeepalive = 25
   ```

   **Peer B**

   ```ini
   [Interface]
   PrivateKey = <peer B private key>
   Address    = 10.0.0.2/32

   [Peer]
   PublicKey  = <peer A public key>
   Endpoint   = <Wg-Knot server IP>:<Wg-Knot port>
   AllowedIPs = 0.0.0.0/0   # Route all traffic through peer A
   PersistentKeepalive = 25
   ```

---

# Wg-Knot

Wg-Knot は、NAT 環境下にある 2 つの WireGuard ピア間で **途中で復号することなく** トラフィックを中継できるリレー プログラムです。

WireGuardのクライアントに変更を加えることなく利用することができます。

## 使い方

```bash
# ビルド
go build -o wg-knot .

# 設定ファイルを作成
cp setting.conf.example setting.conf

# 実行
./wg-knot
```


## 設定

### 設定ファイル

まずは `setting.conf.example` をコピーし、用途に合わせて編集してください。

### 環境変数

| 変数名                      | 説明                                 | 既定値              |
| ------------------------ | ---------------------------------- | ---------------- |
| `WG_KNOT_CONFIG_FILE`    | 設定ファイルへのパス                         | `./setting.conf` |
| `WG_KNOT_LISTEN_ADDRESS` | 受信待ち受け IP アドレス                     | `0.0.0.0`        |
| `WG_KNOT_PORT`           | 受信待ち受け UDP ポート                     | `52820`          |
| `WG_KNOT_LOG_LEVEL`      | ログレベル (`debug`, `info`, `warn` など) | `info`           |

### コマンドラインフラグ

| フラグ           | 説明             |
| ------------- | -------------- |
| `-configfile` | 設定ファイルへのパス     |
| `-listen`     | 受信待ち受け IP アドレス |
| `-port`       | 受信待ち受け UDP ポート |
| `-loglevel`   | ログレベル          |


## 使用例

1. **Wg-Knot を起動する**

   ```bash
   ./wg-knot
   ```

   最小構成の `setting.conf`:

   ```
   [server]
   port = <Wg-Knot が待ち受けるポート>

   # 許可する公開鍵のペア
   [[keypairs]]
   key1 = "<ピア A の公開鍵>"
   key2 = "<ピア B の公開鍵>"
   ```

2. **各ピアを設定して接続する**

   **ピア A**

   ```ini
   [Interface]
   PrivateKey = <ピア A の秘密鍵>
   Address    = 10.0.0.1/32

   [Peer]
   PublicKey  = <ピア B の公開鍵>
   Endpoint   = <Wg-Knot サーバ IP>:<Wg-Knot ポート>
   AllowedIPs = 10.0.0.2/32
   PersistentKeepalive = 25
   ```

   **ピア B**

   ```ini
   [Interface]
   PrivateKey = <ピア B の秘密鍵>
   Address    = 10.0.0.2/32

   [Peer]
   PublicKey  = <ピア A の公開鍵>
   Endpoint   = <Wg-Knot サーバ IP>:<Wg-Knot ポート>
   AllowedIPs = 0.0.0.0/0   # すべてのトラフィックをピア A 経由でルーティング
   PersistentKeepalive = 25
   ```
