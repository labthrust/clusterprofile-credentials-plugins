# eks-aws-auth-plugin

Amazon EKS 向けの Exec Credentials プラグインです。`kubectl`/`kubeconfig` から呼び出され、EKS の IAM ベースのトークンを取得して ExecCredential JSON を標準出力に返します。

このプラグインはリポジトリ直下の `eks-aws-auth-plugin.sh` と同等の挙動を Go で実装したものです。

## 要件

- `aws` CLI が `PATH` 上に存在し、EKS API に必要な権限で認証済みであること
- kubeconfig の `exec` 設定で `provideClusterInfo: true` が設定されていること（`KUBERNETES_EXEC_INFO` を受け取るため）

## ビルド

リポジトリルートで実行してください。

```bash
# 単一バイナリのビルド
GOOS=$(go env GOOS) GOARCH=$(go env GOARCH) \
  go build -o bin/eks-aws-auth-plugin ./cmd/eks-aws-auth-plugin

# 例: Linux/amd64 へクロスコンパイル
GOOS=linux GOARCH=amd64 \
  go build -o bin/eks-aws-auth-plugin-linux-amd64 ./cmd/eks-aws-auth-plugin
```

## 使い方（kubeconfig）

```yaml
users:
  - name: eks-aws-auth
    user:
      exec:
        apiVersion: client.authentication.k8s.io/v1beta1
        command: /path/to/eks-aws-auth-plugin
        provideClusterInfo: true
```

- `kubectl` 実行時に、`KUBERNETES_EXEC_INFO` からクラスタの `server` を読み取り、リージョンを推定します。
- 指定リージョン内の EKS クラスタを列挙し、`describe-cluster` で得られる `endpoint` を比較して、対象クラスタ名を特定します（キャッシュ有り）。
- `aws eks get-token` を呼び出して取得した JSON を、そのまま標準出力に返します。

## キャッシュ

エンドポイント正規化値 → クラスタ名 の簡易マップを JSON で保存します。

- 位置: `${XDG_CACHE_HOME:-$HOME/.cache}/eks-exec-credential/endpoint-map-<region>.json`
- キャッシュがヒットすれば列挙をスキップし、高速化します。

## 動作のポイント

- サーバURLは以下のように正規化して比較します:
  - スキーム削除（`https://`/`http://`）
  - 末尾の `/` を除去
  - ポート `:443` を除去
- リージョンは `...<suffix>.<region>.eks.amazonaws.com`（`-fips` や `.cn` を含むパターンに対応）から推定します。

## ライセンス

このディレクトリのコードはリポジトリルートの `LICENSE` に従います。
