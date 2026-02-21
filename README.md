# spine-simple-chat-demo

`spine` 프레임워크 기반의 WebSocket 채팅 예제 프로젝트입니다.  
클라이언트가 `/ws/chat`에 연결해 메시지를 보내면, 서버가 모든 연결된 클라이언트에게 브로드캐스트합니다.

## Features

- WebSocket 채팅 엔드포인트: `ws://localhost:8080/ws/chat`
- 입력 메시지 공백 제거 및 빈 메시지 무시
- 발신자 ID, UTC 타임스탬프를 포함한 이벤트 브로드캐스트
- 전송 실패한 연결 자동 정리
- Graceful shutdown 지원 (타임아웃 10초)

## Requirements

- Go `1.25.5` 이상

## Run

```bash
go mod tidy
go run .
```

기본 서버 주소는 `:8080`입니다.

## WebSocket API

### Connect

- URL: `ws://localhost:8080/ws/chat`

### Client -> Server

```json
{
  "message": "hello everyone"
}
```

### Server -> Clients (broadcast event)

```json
{
  "type": "message",
  "from": "connection-id",
  "message": "hello everyone",
  "at": "2026-02-21T10:00:00Z"
}
```

필드 설명:

- `type`: 이벤트 타입 (현재 `"message"` 고정)
- `from`: 서버가 부여한 발신자 연결 ID
- `message`: trim 처리된 메시지 본문
- `at`: UTC 기준 RFC3339 타임스탬프

## Quick Test

예: `wscat` 사용

```bash
# terminal 1
go run .

# terminal 2
wscat -c ws://localhost:8080/ws/chat

# terminal 3
wscat -c ws://localhost:8080/ws/chat
```

한 터미널에서 아래처럼 전송:

```json
{"message":"hi"}
```

두 클라이언트 모두 브로드캐스트 이벤트를 받으면 정상입니다.

## Project Structure

```text
.
├── main.go                     # 앱 부트스트랩 및 서버 실행
├── routes/chat_routes.go       # WebSocket 라우트 등록
└── controller/chat_controller.go # 채팅 메시지 처리/브로드캐스트
```
