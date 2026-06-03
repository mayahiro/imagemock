# imagemock

`imagemock` は、ローカル開発、テスト、モック連携向けの小さなダミー画像 HTTP サーバーです

すべての request に対して、生成した単色画像を返します。request path は無視されるため、`/avatar`、`/images/1`、`/anything/else` は同じように処理されます

## インストール

local checkout から install する場合:

```sh
go install .
```

module path から install する場合:

```sh
go install github.com/mayahiro/imagemock@latest
```

installed binary は `GOBIN`、`GOBIN` が未設定の場合は `GOPATH/bin` に出力されます。この directory を `PATH` に含めてください

## 使い方

```sh
go run . --port 8080
```

```sh
go run . \
  --port 8080 \
  --width-min 200 \
  --width-max 400 \
  --height-min 100 \
  --height-max 300 \
  --aspect-ratio 16:9,4:3 \
  --color 80ff0000 \
  --format webp \
  --cache-control 60 \
  --seed 12345
```

任意の path にアクセスできます

```sh
curl 'http://localhost:8080/any/path?width=320&height=240&color=0f0&format=png' --output image.png
```

## CLI オプション

| オプション | 説明 | 既定値 |
| --- | --- | --- |
| `--port` | 待ち受けポート | `8080` |
| `--width-min` | 生成する幅の最小値 | `1` |
| `--width-max` | 生成する幅の最大値 | `1024` |
| `--height-min` | 生成する高さの最小値 | `1` |
| `--height-max` | 生成する高さの最大値 | `1024` |
| `--aspect-ratio` | `16:9` のような生成候補の aspect ratio。option を繰り返すか comma-separated で複数指定できます | 任意 |
| `--color` | `RGB`、`RRGGBB`、`AARRGGBB` 形式の固定色 | request ごとにランダムな不透明 RGB |
| `--format` | 固定出力形式: `jpg`、`png`、`webp` | request ごとにランダム |
| `--cache-control` | browser cache duration の秒数、または `no-store` にする `none` | `60` |
| `--no-label` | 中央の `w x h` label を無効化します | label 有効 |
| `--quality` | `1` から `100` の JPEG quality | `80` |
| `--seed` | 生成される寸法、色、format の deterministic random seed | ランダム |

幅、高さ、色、フォーマットが CLI オプションで固定されていない場合、それぞれ request ごとに個別に決定されます

寸法は `1..16384` に制限されます。WebP lossless の形式上の上限内に収め、意図しない巨大画像を避けるためです

aspect ratio が設定されている場合、server は request ごとに候補から 1 つ選び、設定された幅と高さの範囲に収まる整数倍の size を生成します。query の `width` と `height` が明示されている場合は、aspect-ratio 生成より優先されます

生成されるすべての画像の中央には、embedded Go Regular font を使って `w x h` 形式でサイズが印字されます。文字色と文字サイズは画像の寸法と背景色から決定されます

`--quality` は現在 `jpg` 出力にのみ影響します。`webp` 出力は `github.com/mayahiro/go-webp` が現在 VP8L lossless image を出力するため lossless です

## クエリオプション

対応する query option は次の通りです

| クエリ | 説明 |
| --- | --- |
| `width` | 要求する幅。設定された幅の範囲に clamp されます |
| `height` | 要求する高さ。設定された高さの範囲に clamp されます |
| `color` | `RGB`、`RRGGBB`、`AARRGGBB` 形式の要求色 |
| `format` | 要求する形式: `jpg`、`png`、`webp` |

未対応または不正な query value は無視されます

短い alias も利用できます

| 正式名 | alias |
| --- | --- |
| `width` | `w` |
| `height` | `h` |
| `color` | `c`, `bg` |
| `format` | `fmt`, `f` |

## フォーマット

- `jpg`: Go standard library の JPEG encoder で出力します。JPEG には alpha channel がないため alpha は無視されます
- `png`: 単色 RGBA PNG として出力します
- `webp`: `github.com/mayahiro/go-webp` で lossless VP8L WebP image として出力します

## シャットダウン

server は `SIGINT` と `SIGTERM` を受け取ると graceful shutdown します

## 確認

```sh
go tool goimports -w .
go vet ./...
go test ./...
```

## ライセンス

[LICENSE](LICENSE) を参照してください
