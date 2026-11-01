# spine-simple-chat-demo

Spine `v0.5.1`로 인증된 WebSocket 채팅을 구현하는 예제입니다. 명시적인 `join/message/leave` 프로토콜, 방별 브로드캐스트, handshake 인증, 연결·메시지 크기·timeout 제한과 graceful shutdown을 함께 보여줍니다.

## 실행

요구 사항은 Go `1.27.3` 이상입니다. Go 1.27.x 검증 기준은 `1.27.3`입니다.

```bash
go run .
```

기본 주소는 `:8080`, 데모 토큰은 `demo-token`입니다. `wscat`으로 바로 연결할 수 있습니다.

```bash
wscat -c 'ws://localhost:8080/ws/chat?token=demo-token' \
  -H 'Origin: http://localhost:8080'
```

운영 환경에서는 반드시 `CHAT_AUTH_TOKEN`을 별도 비밀값으로 지정하고 TLS(`wss://`)를 사용하십시오. URL query의 토큰은 브라우저 WebSocket API를 위한 데모 경로이며 접근 로그에 남을 수 있습니다. 헤더를 설정할 수 있는 클라이언트는 `Authorization: Bearer <token>`을 사용하십시오.

## 프로토콜

연결한 클라이언트는 먼저 방에 참여해야 합니다. 방과 사용자 이름에는 영문자, 숫자, `_`, `-`만 사용할 수 있으며 최대 32자입니다.

```json
{"type":"join","room":"general","user":"alice"}
```

같은 방에는 같은 사용자 이름으로 중복 참여할 수 없고, 한 연결이 여러 방에 동시에 참여할 수도 없습니다. 참여하면 방 구성원 모두가 다음 이벤트를 받습니다.

```json
{"type":"joined","room":"general","from":"alice","at":"2026-07-26T10:00:00Z"}
```

참여 후 최대 1000자의 메시지를 보낼 수 있습니다. 본문의 앞뒤 공백은 제거됩니다.

```json
{"type":"message","message":"hello everyone"}
```

```json
{"type":"message","room":"general","from":"alice","message":"hello everyone","at":"2026-07-26T10:00:01Z"}
```

퇴장은 명시적인 명령입니다. 퇴장한 연결과 남은 방 구성원이 모두 `left` 이벤트를 받습니다.

```json
{"type":"leave"}
```

```json
{"type":"left","room":"general","from":"alice","at":"2026-07-26T10:00:02Z"}
```

잘못된 상태나 입력은 연결을 끊지 않고 구조화된 오류 이벤트로 반환합니다.

```json
{"type":"error","message":"join a room before sending messages","code":"NOT_JOINED","at":"2026-07-26T10:00:03Z"}
```

주요 코드는 `INVALID_COMMAND`, `INVALID_ROOM`, `INVALID_USER`, `INVALID_MESSAGE`, `NOT_JOINED`, `ALREADY_JOINED`, `USER_ALREADY_EXISTS`입니다.

## 보안 및 런타임 설정

`AuthInterceptor.PreHandshake`가 연결 슬롯 예약 후 HTTP upgrade 전에 토큰을 검사합니다. 인증 실패는 HTTP 401이며, 허용되지 않은 브라우저 출처는 upgrade 단계에서 HTTP 403입니다.

| 환경변수 | 기본값 | 설명 |
|---|---:|---|
| `CHAT_ADDRESS` | `:8080` | listener 주소 |
| `CHAT_AUTH_TOKEN` | `demo-token` | handshake 토큰 |
| `CHAT_ALLOWED_ORIGINS` | 비어 있음 | 쉼표로 구분한 정확한 출처. 비어 있으면 요청 host 기준 기본 검사 |
| `CHAT_MAX_CONNECTIONS` | `256` | 동시 연결 제한 |
| `CHAT_MAX_MESSAGE_BYTES` | `4096` | WebSocket frame 읽기 제한 |
| `CHAT_HANDSHAKE_TIMEOUT` | `5s` | handshake 쓰기 제한 시간 |
| `CHAT_READ_TIMEOUT` | `60s` | 메시지 또는 pong 대기 시간 |
| `CHAT_WRITE_TIMEOUT` | `5s` | 메시지·제어 frame 쓰기 시간 |
| `CHAT_PING_INTERVAL` | `25s` | 서버 ping 간격 |
| `CHAT_SHUTDOWN_TIMEOUT` | `10s` | graceful shutdown 제한 시간 |

예를 들어 별도 프런트엔드 두 곳을 허용하려면 다음과 같이 실행합니다.

```bash
CHAT_AUTH_TOKEN='replace-me' \
CHAT_ALLOWED_ORIGINS='https://chat.example.com,https://admin.example.com' \
go run .
```

Spine `v0.5.x`의 `App.Validate`가 잘못된 옵션을 listener 시작 전에 검사합니다. 연결 용량을 넘으면 HTTP 503과 `Retry-After`, `WEBSOCKET_CAPACITY_EXCEEDED`가 반환되고, 크기 제한을 넘는 frame은 close code 1009로 종료됩니다. 종료 신호를 받으면 새 연결을 거부하고 활성 handler를 drain한 뒤 WebSocket 런타임을 종료합니다.

Spine에는 현재 애플리케이션 수준의 `OnConnect`/`OnDisconnect` 콜백이 없습니다. 이 예제는 연결 여부를 추측하지 않고 `join`과 `leave`를 프로토콜로 명시하며, 비정상 종료된 연결은 다음 전송 실패 시 방 상태에서 제거합니다. 따라서 연결 종료를 즉시 영속화해야 하는 시스템은 별도의 presence 저장소와 만료 정책이 필요합니다.

## 테스트

단위·race·정적 검사를 실행합니다.

```bash
go test ./...
go test -race ./...
go vet ./...
```

실제 listener E2E는 실행 중인 앱을 대상으로 인증, 출처, 방 격리, 상태 오류, 중복 사용자, 용량 제한과 메시지 크기 제한을 검증합니다.

```bash
# terminal 1
go run .

# terminal 2
go test -tags=e2e -v ./e2e
```

서버 설정을 바꿨다면 E2E에도 대응 값을 전달합니다.

```bash
CHAT_E2E_WS_URL='ws://127.0.0.1:9080/ws/chat' \
CHAT_E2E_ORIGIN='https://chat.example.com' \
CHAT_E2E_AUTH_TOKEN='replace-me' \
CHAT_E2E_MAX_CONNECTIONS=32 \
CHAT_E2E_MAX_MESSAGE_BYTES=2048 \
go test -tags=e2e -v ./e2e
```

## 구조

```text
.
├── config/                    # 환경변수와 안전 제한 설정
├── controller/                # 방 상태와 채팅 프로토콜
├── interceptor/               # upgrade 전 토큰 인증
├── routes/                    # WebSocket route 등록
├── e2e/                       # 실제 listener 재사용 E2E
└── main.go                    # DI, 검증, 실행 및 graceful shutdown
```
