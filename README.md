# imagemock

`imagemock` is a small dummy image HTTP server for local development, testing, and mock integrations.

It returns a generated solid-color image for every request. The request path is ignored, so `/avatar`, `/images/1`, and `/anything/else` are handled the same way.

## Installation

From a local checkout:

```sh
go install .
```

From the module path:

```sh
go install github.com/mayahiro/imagemock@latest
```

The installed binary is written to `GOBIN`, or to `GOPATH/bin` when `GOBIN` is not set. Make sure that directory is included in `PATH`.

## Usage

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
  --cache-control 60
```

Open any path on the server:

```sh
curl 'http://localhost:8080/any/path?width=320&height=240&color=0f0&format=png' --output image.png
```

## CLI Options

| Option | Description | Default |
| --- | --- | --- |
| `--port` | Listening port | `8080` |
| `--width-min` | Minimum generated width | `1` |
| `--width-max` | Maximum generated width | `1024` |
| `--height-min` | Minimum generated height | `1` |
| `--height-max` | Maximum generated height | `1024` |
| `--aspect-ratio` | Allowed generated aspect ratio such as `16:9`; repeat the option or separate values with commas | Any ratio |
| `--color` | Fixed color as `RGB`, `RRGGBB`, or `AARRGGBB` hex | Random opaque RGB per request |
| `--format` | Fixed output format: `jpg`, `png`, or `webp` | Random per request |
| `--cache-control` | Browser cache duration in seconds, or `none` for `no-store` | `60` |

When width, height, color, or format are not fixed by CLI options, they are selected independently for each request.

Dimensions are limited to `1..16384`. This keeps the WebP lossless output inside the format limit and avoids accidental oversized images.

When aspect ratios are configured, the server chooses one ratio per request and generates an integer-multiple size within the configured width and height ranges. Explicit `width` and `height` query values take precedence over aspect-ratio generation.

Every generated image prints its size at the center as `w x h` using the embedded Go Regular font. The text color and size are selected from the image dimensions and background color.

## Query Options

Supported query options are:

| Query | Description |
| --- | --- |
| `width` | Requested width, clamped to the configured width range |
| `height` | Requested height, clamped to the configured height range |
| `color` | Requested color as `RGB`, `RRGGBB`, or `AARRGGBB` hex |
| `format` | Requested format: `jpg`, `png`, or `webp` |

Unsupported or invalid query values are ignored.

## Formats

- `jpg`: encoded with the Go standard library JPEG encoder. Alpha is ignored because JPEG has no alpha channel
- `png`: encoded as a solid-color RGBA PNG
- `webp`: encoded as a lossless VP8L WebP image with `github.com/mayahiro/go-webp`

## Shutdown

The server handles `SIGINT` and `SIGTERM` with a graceful shutdown.

## Verification

```sh
go tool goimports -w .
go vet ./...
go test ./...
```

## License

See [LICENSE](LICENSE).
