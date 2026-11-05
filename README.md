# Spine WebSocket 채팅 예제

인증된 클라이언트가 방에 참여하고 메시지를 주고받는 예제입니다. Spine `v0.7.0-preview.1`과 CLI `v0.4.3-preview.2`를 사용합니다.

WebSocket 서버에서는 연결 업그레이드, 인증, JSON 해석, 방 상태, 응답 전송과 종료 처리가 한 핸들러에 섞이기 쉽습니다. 이 예제는 **메시지를 해석하고 실행하는 공통 흐름을 Spine에 맡기고, 방의 규칙을 서비스에 모읍니다.** 방별 전송, 중복 이름 거부, 연결 종료 후 이름 재사용까지 실제 연결로 확인할 수 있습니다.

## 예제 채널 선택

| 채널 | 브랜치 | 용도 |
| --- | --- | --- |
| Preview | [`preview`](https://github.com/NARUBROWN/spine-simple-chat-demo/tree/preview) | 개발 중인 변경과 실험 버전을 확인합니다. |
| RC | [`rc`](https://github.com/NARUBROWN/spine-simple-chat-demo/tree/rc) | 정식 공개 전 릴리스 후보를 확인합니다. |
| Stable | [`main`](https://github.com/NARUBROWN/spine-simple-chat-demo/tree/main) | 정식 의존성을 사용하는 공개 예제이며 기본 브랜치입니다. |

예제 코드는 `preview → rc → main` 순서로 승격합니다. 예제 태그는 `vX.Y.Z-preview.N`, `vX.Y.Z-rc.N`, `vX.Y.Z`로 구분하며, **GitHub Release는 Stable에서만 발행합니다.** 실제 Spine 버전은 선택한 브랜치의 `go.mod`를 확인하세요. [채널 검사와 발행 절차](RELEASE_CHANNELS.md)를 참고하세요.

## 코드에서 볼 부분

```text
HTTP handshake → AuthInterceptor → Echo 업그레이드 + Gorilla 연결
                                          │
                    Spine의 JSON 바인딩 · 인자 해석 · DI
                                          │
                           ChatSocket.Handle
                                          │
                              RoomService
                                          │
                         ChatEvent → 연결별 전송
```

| 책임 | 코드 |
|---|---|
| HTTP와 WebSocket 구현 선택, 생성자·인터셉터 등록 | [BuildApplication](internal/application/application.go) |
| 업그레이드 전 토큰 검사 | [AuthInterceptor](internal/interceptor/auth_interceptor.go) |
| JSON 입력과 출력의 형태 | [ChatMessage / ChatEvent](src/dto/chat_message.go) |
| 해석된 입력과 연결을 서비스에 전달 | [ChatSocket](src/websocket/chat/chat_handler.go) |
| 참여·퇴장·이름 충돌·방별 전송·참여 상태 정리 | [RoomService](src/service/room_service.go) |
| 출처·연결 수·메시지 크기·시간 제한 | [Config](internal/config/config.go) |

핸들러는 요청을 다음 시그니처로 받습니다.

```go
func (c *ChatSocket) Handle(ctx context.Context, connID ws.ConnectionID, command dto.ChatMessage) error
```

Spine이 `context.Context`, 연결 ID와 JSON DTO를 해석하고, 생성자로 등록한 `RoomService`를 핸들러에 주입합니다. 핸들러에는 업그레이더나 읽기 루프가 없습니다. `RoomService`도 Spine·Echo·Gorilla를 import하지 않으며, 이벤트를 전달하는 함수만 받습니다. 따라서 방 규칙은 소켓 없이 테스트할 수 있습니다.

`BuildApplication`은 구현과 정책을 명시적으로 연결합니다.

```go
app := spine.New(echoplatform.Option(), gorillaplatform.Option())
app.Constructor(service.NewRoomService)
app.InterceptorFor(boot.InterceptorWebSocket, interceptor.NewAuthInterceptor(cfg.AuthToken))
```

HTTP는 업그레이드 경로를, Gorilla는 프레임 입출력을 제공합니다. Spine은 인자 해석, 인터셉터 실행, 연결 제한과 종료 흐름을 관리합니다. 인증 기준과 채팅 규칙은 애플리케이션이 정합니다.

## 실행하기

Go `1.25.5` 이상이 필요하며, 로컬 검증에는 Go `1.26.0`을 사용합니다. DB와 브로커는 필요하지 않습니다.

```sh
go install github.com/NARUBROWN/spine-cli@v0.4.3-preview.2
spine-cli version
# spine-cli 0.4.3-preview.2
go mod download all
spine-cli dev
```

코드를 수정하면 CLI가 다시 빌드하고 빌드 성공 후 서버를 교체합니다. 기본 주소는 `:8080`입니다. `CHAT_ADDRESS`를 생략하면 CLI가 전달하는 `SPINE_PORT`를 사용합니다.

다른 터미널에서 연결합니다. 아래 명령은 `wscat`이 설치되어 있다고 가정합니다.

```sh
wscat -c 'ws://localhost:8080/ws/chat?token=demo-token' \
  -H 'Origin: http://localhost:8080'
```

실행 코어는 Spine `v0.7.0-preview.1`, HTTP 플랫폼은 `spine-echo-platform v0.2.0-preview.1`, WebSocket 플랫폼은 `spine-gorilla-platform v0.2.0-preview.1`입니다. Echo는 `v5.4.4`입니다.

## 방에 참여하고 메시지 보내기

연결한 뒤 다음 JSON을 순서대로 입력합니다.

```json
{"type":"join","room":"general","user":"alice"}
{"type":"message","message":"hello everyone"}
{"type":"leave"}
```

참여하면 같은 방 구성원 모두가 `joined`를 받고, 메시지는 발신자를 포함한 같은 방 구성원에게만 전달됩니다. 퇴장한 연결과 남은 구성원은 모두 `left`를 받습니다.

```json
{"type":"joined","room":"general","from":"alice","at":"2026-11-03T00:00:00Z"}
{"type":"message","room":"general","from":"alice","message":"hello everyone","at":"2026-11-03T00:00:01Z"}
{"type":"left","room":"general","from":"alice","at":"2026-11-03T00:00:02Z"}
```

두 번째 클라이언트를 같은 방의 `bob`으로 연결해 전송을 확인하고, 세 번째 클라이언트는 다른 방에 참여시켜 방 격리를 확인할 수 있습니다.

- 방과 이름은 영문자·숫자·`_`·`-`로 구성하며 최대 32자입니다.
- 같은 방의 이름은 중복될 수 없고, 한 연결은 한 방에만 참여합니다.
- 메시지는 앞뒤 공백을 제거한 뒤 1~1000자여야 합니다.
- 연결 context가 취소되면 참여 상태를 정리합니다. 이전 참여의 늦은 정리가 새 참여를 지우지 않도록 참여별 식별자를 사용합니다.

잘못된 명령이나 참여 상태는 연결을 유지한 채 오류 이벤트로 알립니다.

```json
{"type":"error","message":"join a room before sending messages","code":"NOT_JOINED","at":"2026-11-03T00:00:03Z"}
```

주요 코드는 `INVALID_COMMAND`, `INVALID_ROOM`, `INVALID_USER`, `INVALID_MESSAGE`, `NOT_JOINED`, `ALREADY_JOINED`, `USER_ALREADY_EXISTS`입니다. 이 프로토콜을 유지하기 위해 DTO 검증기는 선택하지 않았습니다(`validator = "none"`). 입력 태그 위반 시 연결을 닫는 공통 검증 정책이 필요하면 검증기를 별도로 선택할 수 있습니다.

## 연결 정책과 설정

인증 실패는 업그레이드 전 HTTP 401, 허용되지 않은 출처는 HTTP 403입니다. 연결 용량을 넘으면 HTTP 503과 `Retry-After`, `WEBSOCKET_CAPACITY_EXCEEDED`를 반환합니다. 크기 제한을 넘는 메시지는 close code 1009로 종료됩니다. 옵션 오류는 `App.Validate`에서 리스너 시작 전에 확인합니다.

| 환경변수 | 기본값 | 의미 |
|---|---|---|
| `CHAT_ADDRESS` | `SPINE_PORT` 또는 `:8080` | listener 주소 |
| `CHAT_AUTH_TOKEN` | `demo-token` | handshake 토큰 |
| `CHAT_ALLOWED_ORIGINS` | 비어 있음 | 정확한 출처의 쉼표 구분 목록. 비어 있으면 요청 host 기준 검사 |
| `CHAT_MAX_CONNECTIONS` | `256` | 동시 연결 제한 |
| `CHAT_MAX_MESSAGE_BYTES` | `4096` | 메시지 읽기 크기 제한 |
| `CHAT_HANDSHAKE_TIMEOUT` | `5s` | handshake 쓰기 제한 시간 |
| `CHAT_READ_TIMEOUT` | `60s` | 메시지 또는 pong 대기 시간 |
| `CHAT_WRITE_TIMEOUT` | `5s` | 메시지·제어 프레임 쓰기 시간 |
| `CHAT_PING_INTERVAL` | `25s` | 서버 ping 간격 |
| `CHAT_SHUTDOWN_TIMEOUT` | `10s` | 종료 대기 제한 시간 |

```sh
CHAT_AUTH_TOKEN='replace-me' \
CHAT_ALLOWED_ORIGINS='https://chat.example.com,https://admin.example.com' \
spine-cli dev
```

운영 환경에서는 별도의 비밀 토큰과 TLS(`wss://`)를 사용하십시오. URL query 토큰은 브라우저 WebSocket API를 위한 데모 방식이며 접근 로그에 남을 수 있습니다. 헤더를 설정할 수 있는 클라이언트는 `Authorization: Bearer <token>`을 사용합니다. Bearer가 있으면 query 토큰보다 우선합니다. 공용 데모 토큰과 표시 이름은 사용자별 인증이나 방 접근 권한을 제공하지 않습니다.

종료 신호를 받으면 새 연결을 중단하고 활성 연결·핸들러를 정리합니다. 갑작스러운 연결 종료를 감지하는 시점은 네트워크와 timeout에 따라 달라집니다. 참여 상태는 프로세스 메모리에만 있으므로 다중 서버의 presence 공유에는 별도 저장소와 만료 정책이 필요합니다.

## 검증하기

```sh
spine-cli context --root . --json
spine-cli contract check --root . --warnings-as-errors --json
spine-cli verify --root . --warnings-as-errors --json
spine-cli build --root .
spine-cli contract probe --root . --json
go test -race -count=1 ./...
go test -race -count=1 -tags=e2e -v ./e2e
```

`verify`는 정적 실행 계약·서식·vet·Go 테스트를 검사합니다. 부분 probe는 `BuildApplication`과 `Validate`, 공개 WebSocket registry를 대조하며 리스너를 열지 않습니다.

E2E는 기본적으로 임의 loopback 포트에 전용 서버를 빌드·시작하고 종료합니다. 인증·출처, 방 격리, 이름 충돌·재사용, 실패한 handshake의 슬롯 반환, 잘못된 JSON의 연결 간 격리, 용량·크기 제한을 실제 TCP 연결로 검사합니다.

이미 실행한 전용 서버를 검사하려면 다음 값을 전달합니다. 용량 검사가 연결 슬롯을 채우므로 다른 사용자와 공유하는 서버를 지정하지 마십시오. URL에는 loopback 주소만 허용합니다.

```sh
CHAT_E2E_WS_URL='ws://127.0.0.1:9080/ws/chat' \
CHAT_E2E_ORIGIN='https://chat.example.com' \
CHAT_E2E_AUTH_TOKEN='replace-me' \
CHAT_E2E_MAX_CONNECTIONS=32 \
CHAT_E2E_MAX_MESSAGE_BYTES=2048 \
go test -count=1 -tags=e2e -v ./e2e
```

## CLI로 확장하기

현재 선택은 `spine.toml`의 `platform = "echo"`, `websocket_platform = "gorilla"`, `recipe = "platform-contracts-v0.7.0-preview.1"`에 기록되어 있습니다. 같은 출발점은 `spine-cli new chat-demo --module example.com/chat --websocket-platform gorilla`로 만들 수 있습니다.

추가 소켓의 생성 계획은 다음 명령으로 확인합니다.

```sh
spine-cli g websocket notice --path /ws/notice --field message:string --dry-run --json
```

`changes`, `diff`, `base_hash`를 검토하고 같은 명령의 `--dry-run`을 `--if-match <base_hash>`로 바꿔 적용합니다. 생성된 핸들러의 메시지 처리 로직은 직접 구현합니다.

의존성 갱신은 `spine-cli update --dry-run --json`으로 검토하고 `--if-match`로 적용합니다. 기존 앱에 플랫폼 선택이나 서비스 분리를 자동으로 추가하지는 않습니다. 전체 명령과 분석 한계는 [Spine CLI 문서](https://github.com/NARUBROWN/spine-cli#readme)를 참고하세요.
