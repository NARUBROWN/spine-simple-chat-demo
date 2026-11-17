English | [Korean](README.ko.md)

# Spine WebSocket Chat Example

Authenticated clients join rooms and exchange messages. Uses Spine `v0.7.6-preview.1` and CLI v0.4.5-preview.5.

WebSocket handlers can mix connection upgrades, authentication, JSON decoding, room state, sending, and shutdown. This example **lets Spine handle message interpretation and execution while a service owns room rules**. Real connections demonstrate room-scoped delivery, duplicate-name rejection, and name reuse after disconnect.

## Example channels

| Channel | Branch | Purpose |
| --- | --- | --- |
| Preview | [`preview`](https://github.com/NARUBROWN/spine-simple-chat-demo/tree/preview) | Development changes and experimental versions |
| RC | [`rc`](https://github.com/NARUBROWN/spine-simple-chat-demo/tree/rc) | Release candidates before stable publication |
| Stable | [`main`](https://github.com/NARUBROWN/spine-simple-chat-demo/tree/main) | Public example using stable dependencies; the default branch |

Example code is promoted through `preview → rc → main`. Tags use `vX.Y.Z-preview.N`, `vX.Y.Z-rc.N`, and `vX.Y.Z`. **GitHub Releases are published only for Stable.** Check the selected branch's `go.mod` for the actual Spine version. See [channel checks and publication](RELEASE_CHANNELS.md).

## Code guide

```text
HTTP handshake → AuthInterceptor → Echo upgrade + Gorilla connection
                                          │
                    Spine JSON binding, argument resolution, DI
                                          │
                           ChatSocket.Handle
                                          │
                              RoomService
                                          │
                         ChatEvent → per-connection delivery
```

| Responsibility | Code |
| --- | --- |
| Select HTTP/WebSocket implementations and register services and interceptors | [BuildApplication](internal/application/application.go) |
| Register the WebSocket receiver and route | [Chat handler](src/websocket/chat/chat_handler.go) |
| Check tokens before upgrading | [AuthInterceptor](internal/interceptor/auth_interceptor.go) |
| Define JSON input and output | [ChatMessage / ChatEvent](src/dto/chat_message.go) |
| Pass resolved input and the connection to the service | [ChatSocket](src/websocket/chat/chat_handler.go) |
| Join, leave, name conflicts, room delivery, and membership cleanup | [RoomService](src/service/room_service.go) |
| Origins, connection limits, message size, and timeouts | [Config](internal/config/config.go) |

The handler receives requests through this signature:

```go
func (c *ChatSocket) Handle(ctx spinectx.Ctx, connID ws.ConnectionID, command dto.ChatMessage) error
```

`spinectx` aliases `github.com/NARUBROWN/spine/pkg/spine`. Spine resolves `spinectx.Ctx`, the connection ID, and JSON DTO, then injects the constructor-registered `RoomService`. The handler retrieves the standard context with `ctx.ExecutionContext().Context()` and passes it to the service and `ws.Send`. It contains no upgrader or read loop. `RoomService` also imports no Spine, Echo, or Gorilla packages; it receives only an event delivery function, so room rules can be tested without sockets.

`BuildApplication` explicitly connects implementations and policies.

```go
app := spine.New(echoplatform.Option(), gorillaplatform.Option())
app.Constructor(
    constructor.Service(&service.RoomService{}),
    constructor.Custom(&chat.ChatSocket{}),
    constructor.Interceptor(&interceptor.AuthInterceptor{}),
)
app.InterceptorFor(boot.InterceptorWebSocket, (*interceptor.AuthInterceptor)(nil))
if err := chat.Register(app); err != nil {
    return nil, boot.Options{}, err
}
```

`constructor` is `github.com/NARUBROWN/spine/pkg/constructor`. Components implement `Constructor(dependencies...) *Component`. Constructors are registered in application composition. `src/websocket/chat/chat_handler.go` keeps the receiver and its route registration function, `Register`, together.

```go
func Register(app spine.App) error {
    return app.WebSocket().Register("/ws/chat", (*ChatSocket).Handle)
}
```

WebSocket receivers have no dedicated role, so use `constructor.Custom(&ChatSocket{})`. Application composition registers service, interceptor, and receiver constructors, then calls `chat.Register(app)`.

HTTP provides the upgrade route; Gorilla provides frame input/output. Spine manages argument resolution, interceptor execution, connection limits, and shutdown. The application defines authentication and room rules.

## Running

Requires Go `1.25.5` or later; local verification uses Go `1.26.0`. No database or broker is needed.

```sh
go install github.com/NARUBROWN/spine-cli/cmd/spine@v0.4.5-preview.5
spine version
go mod download all
spine dev
```

Install Preview CLI `v0.4.5-preview.5`. CI pins the verified version in `.github/example-channels.json`.

The CLI rebuilds after code changes and replaces the server after a successful build. The default address is `:8080`. If `CHAT_ADDRESS` is unset, the application uses `SPINE_PORT` from the CLI.

Connect in another terminal. The commands below assume `wscat` is installed.

```sh
wscat -c 'ws://localhost:8080/ws/chat?token=demo-token' \
  -H 'Origin: http://localhost:8080'
```

The runtime uses Spine `v0.7.6-preview.1`, `spine-echo-platform v0.2.1-preview.1`, and `spine-gorilla-platform v0.2.0-preview.2`. Echo is `v5.4.4`.

## Join a room and send messages

After connecting, enter the following JSON messages in order.

```json
{"type":"join","room":"general","user":"alice"}
{"type":"message","message":"hello everyone"}
{"type":"leave"}
```

Joining sends `joined` to all members of the room. Messages are delivered only within that room, including to the sender. Both the leaving connection and remaining members receive `left`.

```json
{"type":"joined","room":"general","from":"alice","at":"2026-11-03T00:00:00Z"}
{"type":"message","room":"general","from":"alice","message":"hello everyone","at":"2026-11-03T00:00:01Z"}
{"type":"left","room":"general","from":"alice","at":"2026-11-03T00:00:02Z"}
```

Connect a second client as `bob` in the same room to check delivery. Join a different room from a third client to check room isolation.

- Room and user names contain letters, numbers, `_`, and `-`, with a maximum of 32 characters.
- Names must be unique within a room; each connection can join only one room.
- Messages must contain 1–1000 characters after trimming surrounding whitespace.
- Connection context cancellation cleans up membership. Per-membership identifiers prevent delayed cleanup of an old membership from removing a new one.

Invalid commands or membership states produce error events while keeping the connection open.

```json
{"type":"error","message":"join a room before sending messages","code":"NOT_JOINED","at":"2026-11-03T00:00:03Z"}
```

Common codes are `INVALID_COMMAND`, `INVALID_ROOM`, `INVALID_USER`, `INVALID_MESSAGE`, `NOT_JOINED`, `ALREADY_JOINED`, and `USER_ALREADY_EXISTS`. No DTO validator is selected (`validator = "none"`) to preserve this protocol. Select a validator separately if you want the shared policy that closes connections on input tag violations.

## Connection policies and configuration

Authentication failure returns HTTP 401 before upgrading; disallowed origins return HTTP 403. Exceeding connection capacity returns HTTP 503 with `Retry-After` and `WEBSOCKET_CAPACITY_EXCEEDED`. Oversized messages close with code 1009. `App.Validate` checks option errors before opening a listener.

| Environment variable | Default | Purpose |
| --- | --- | --- |
| `CHAT_ADDRESS` | `SPINE_PORT` or `:8080` | Listener address |
| `CHAT_AUTH_TOKEN` | `demo-token` | Handshake token |
| `CHAT_ALLOWED_ORIGINS` | Empty | Comma-separated exact origins; empty uses the request host check |
| `CHAT_MAX_CONNECTIONS` | `256` | Concurrent connection limit |
| `CHAT_MAX_MESSAGE_BYTES` | `4096` | Message read size limit |
| `CHAT_HANDSHAKE_TIMEOUT` | `5s` | Handshake write timeout |
| `CHAT_READ_TIMEOUT` | `60s` | Message or pong wait timeout |
| `CHAT_WRITE_TIMEOUT` | `5s` | Message/control frame write timeout |
| `CHAT_PING_INTERVAL` | `25s` | Server ping interval |
| `CHAT_SHUTDOWN_TIMEOUT` | `10s` | Shutdown wait timeout |

```sh
CHAT_AUTH_TOKEN='replace-me' \
CHAT_ALLOWED_ORIGINS='https://chat.example.com,https://admin.example.com' \
spine dev
```

Use a separate secret token and TLS (`wss://`) in production. Query-string tokens are a demonstration for browser WebSocket APIs and may appear in access logs. Clients that can set headers should use `Authorization: Bearer <token>`. Bearer tokens take precedence over query tokens. The shared demo token and display names do not provide per-user authentication or room authorization.

On a shutdown signal, the server stops new connections and cleans up active connections and handlers. Detection of abrupt disconnects depends on the network and timeouts. Membership exists only in process memory; shared presence across servers needs separate storage and expiry policies.

## Verification

```sh
spine context --root . --json
spine contract check --root . --warnings-as-errors --json
spine verify --root . --warnings-as-errors --json
spine build --root .
spine contract probe --root . --json
go test -race -count=1 ./...
go test -race -count=1 -tags=e2e -v ./e2e
```

`verify` checks static execution contracts, formatting, vet, and Go tests. Partial probes compare `BuildApplication`, `Validate`, and the public WebSocket registry without opening a listener.

By default, E2E builds, starts, and stops a dedicated server on a random loopback port. Real TCP connections check authentication, origins, room isolation, name conflicts and reuse, capacity slot release after failed handshakes, isolation of invalid JSON between connections, capacity limits, and message size limits.

To test an existing dedicated server, pass the following settings. Capacity tests fill connection slots; do not use a server shared with other users. Only loopback URLs are accepted.

```sh
CHAT_E2E_WS_URL='ws://127.0.0.1:9080/ws/chat' \
CHAT_E2E_ORIGIN='https://chat.example.com' \
CHAT_E2E_AUTH_TOKEN='replace-me' \
CHAT_E2E_MAX_CONNECTIONS=32 \
CHAT_E2E_MAX_MESSAGE_BYTES=2048 \
go test -count=1 -tags=e2e -v ./e2e
```

## Extend with the CLI

`spine.toml` records `platform = "echo"`, `websocket_platform = "gorilla"`, and `recipe = "platform-contracts-preview"`. Generate the same platform selection with `spine new chat-demo --module example.com/chat --websocket-platform gorilla`.

Preview an additional socket's generation plan:

```sh
spine g websocket notice --path /ws/notice --field message:string --dry-run --json
```

Review `changes`, `diff`, and `base_hash`. Apply the same command by replacing `--dry-run` with `--if-match <base_hash>`. Implement the generated handler's message processing yourself.

Review dependency updates with `spine update --dry-run --json`, then apply with `--if-match`. This does not automatically add platform selections or split services in existing applications. See the [Spine CLI documentation](https://github.com/NARUBROWN/spine-cli#readme) for all commands and analysis limits.
