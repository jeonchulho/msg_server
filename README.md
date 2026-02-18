# msg_server

Golang 기반 대용량 메신저 서버(MVP 백엔드 골격)입니다.

- 릴리즈 노트: [RELEASE_NOTES.md](RELEASE_NOTES.md)
- 대용량 준비도 점검표: [SCALABILITY_CHECKLIST.md](SCALABILITY_CHECKLIST.md)
- 10만 사용자 부하 테스트 계획: [LOAD_TEST_100K_PLAN.md](LOAD_TEST_100K_PLAN.md)
- MSA 전환 로드맵: [MSA_ROADMAP.md](MSA_ROADMAP.md)
- MSA 상세 가이드(dbman/Redis): [MSA_DEEPDIVE_DBMAN_REDIS.md](MSA_DEEPDIVE_DBMAN_REDIS.md)
- 아키텍처 구성도: [ARCHITECTURE.md](ARCHITECTURE.md)
- Kubernetes 배포 골격: [k8s/README.md](k8s/README.md)

Kubernetes 환경별 배포 예시:
- 개발: `kubectl apply -k ./k8s/overlays/dev`
- 스테이징: `kubectl apply -k ./k8s/overlays/staging`
- 운영: `kubectl apply -k ./k8s/overlays/prod`

구성 스택:
- LavinMQ (이벤트 브로커)
- Redis Pub/Sub (실시간 팬아웃)
- WebSocket (채팅/시그널링)
- PostgreSQL (주 데이터)
- Milvus (채팅 검색 벡터 인덱싱 골격)
- MinIO (파일 저장소 + 이미지 썸네일)

## 서비스 경계(현재)

- `dbman`: DB DML/조회 책임의 단일 진입점
	- chat/user/tenant/file + session(device_session/note/notify) 관련 DB 처리 담당
- `chat`: 채팅 API/실시간 처리 중심, DB 조회/저장은 `dbman` API 경유
- `session`: 인증/사용자/테넌트 + 세션/노트/알림 API 제공, 영속 처리는 `dbman` API 경유
- `fileman`: MinIO 업로드/다운로드/썸네일 처리 담당, 파일 메타 저장 및 tenant MinIO 메타 조회는 `dbman` API 경유

## 구현 범위

현재 저장소에는 아래 기능의 **백엔드 기반 구현**이 포함됩니다.

1. 조직도/사용자 관리, 사용자 상태, 조직/사용자 검색 REST API
2. MinIO Presigned 업로드/다운로드 API + 이미지 업로드 시 서버 썸네일 생성
3. 채팅방/멤버/메시지 API, 이모지 메타 필드 처리, 파일 메타 연동
4. WebSocket 기반 실시간 메시지 및 WebRTC 시그널링 전달(offer/answer/ice)
5. 전체/채팅방별 메시지 검색(PostgreSQL FTS + Milvus 재정렬 훅)
6. 전체 기능 접근을 위한 REST API 엔드포인트

음성/화상통화의 미디어 전송 자체는 WebRTC 시그널링만 제공하며, SFU/MCU(예: LiveKit, Janus)는 별도 구성 대상입니다.

## 빠른 시작

1) 환경 변수

```bash
cp .env.example .env
```

주요 변수 메모:
- `DBMAN_ENDPOINTS`를 우선 사용합니다. (예: `http://localhost:8082,http://localhost:18082`)
- `DBMAN_ENDPOINT` 기본값은 `http://localhost:8082`이며, `DBMAN_ENDPOINTS` 미설정 시 하위 호환으로 사용됩니다.
- `DBMAN_HTTP_TIMEOUT_MS` 기본값은 `5000`입니다. (dbman 요청 타임아웃)
- `DBMAN_FAIL_THRESHOLD` 기본값은 `3`입니다. (endpoint 연속 실패 임계치)
- `DBMAN_COOLDOWN_MS` 기본값은 `10000`입니다. (임계치 도달 endpoint 임시 제외 시간)
- `chat`/`session`/`fileman`은 DB 관련 처리를 이 엔드포인트로 위임합니다.
- `dbman` 자체도 테넌트 메타 provider 경로에서 `DBMAN_ENDPOINT`를 사용할 수 있으며, 미지정 시 내부 shared DB 조회 fallback으로 동작합니다.
- `VECTORMAN_ENDPOINT` 기본값은 `http://localhost:8083`이며, `chat`은 벡터 인덱싱/검색 처리를 이 엔드포인트로 위임합니다.
- `CHAT_USE_MQ` 기본값은 `true`입니다. `false`면 chat은 MQ publish를 생략하고 메시지를 WebSocket(tenant room channel)으로만 fan-out 합니다.
- `SESSION_PORT` 기본값은 `8090`입니다. (`SESSIOND_PORT`는 하위 호환)
- `FILEMAN_PORT` 기본값은 `8081`입니다.
- `DBMAN_PORT` 기본값은 `8082`입니다.
- `VECTORMAN_PORT` 기본값은 `8083`입니다.

2) 인프라 실행

```bash
make up
```

dbman 읽기/쓰기 분리 확장(로컬, HAProxy 경유) 예시:

```bash
docker compose up -d dbman-write dbman-read dbman-lb --scale dbman-write=2 --scale dbman-read=3
curl -s http://localhost:8082/health/ready
```

- 권장: `.env`에 `POSTGRES_WRITE_DSN`, `POSTGRES_READ_DSN`를 분리해 설정하세요.
- 앱 서비스(`chat/session/fileman`)의 `DBMAN_ENDPOINTS`는 `http://dbman-lb:8082` 사용을 권장합니다.

3) 마이그레이션

```bash
export POSTGRES_DSN="postgres://msg:msg@localhost:5432/msg?sslmode=disable"
make migrate
```

관리자 시드(로그인 스모크용):

```bash
make seed-admin
```

4) 서버 실행

```bash
set -a && source .env && set +a
make run-chat
```

세션/알림 전용 서버 실행(별도 프로세스):

```bash
make run-session
```

파일 전용 서버 실행(별도 프로세스):

```bash
make run-fileman
```

DB DML/검색 전용 서버 실행(별도 프로세스):

```bash
make run-dbman
```

벡터 DML/검색 전용 서버 실행(별도 프로세스):

```bash
make run-vectorman
```

포트 기본값/변경 변수는 위 "주요 변수 메모"를 참고하세요.

헬스체크: `GET /health` (호환), `GET /health/live`, `GET /health/ready`

## session (별도 실행 파일)

실행 파일: `cmd/session`

주요 기능:
- 테넌트+사용자 기준 디바이스 세션 로그인/관리
- 사용자 상태 변경 및 WebSocket 실시간 알림
- Note 전송/수신/알림 (`to/cc/bcc`, 멀티파일 메타 포함)
- 채팅방 메시지 수신 알림 이벤트 전송

주요 엔드포인트:
- Public
	- `GET /health`
	- `GET /ws/session?tenant_id=...&user_id=...&session_id=...&session_token=...`
- Auth(Bearer JWT)
	- `POST /api/v1/session/login` (device session 발급/갱신, `allowed_tenants` 지원)
	- `PATCH /api/v1/session/status`
	- `POST /api/v1/notes`
	- `GET /api/v1/notes/inbox?limit=50`
	- `POST /api/v1/notes/:id/read`
	- `POST /api/v1/chat/notify`

## 인증(JWT) 사용

1) 관리자/매니저 사용자 생성

- `POST /api/v1/users` 요청 바디에 `password`, `role` 포함

2) 로그인

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
	-H "Content-Type: application/json" \
	-d '{"tenant_id":"default","email":"admin@example.com","password":"pass1234"}'
```

응답의 `access_token`을 아래처럼 전달:

```bash
-H "Authorization: Bearer <token>"
```

## 주요 API

기본 prefix: `/api/v1`

- 인증
	- `POST /auth/login` (Public)

## 공통 응답 스키마

- 에러 응답
	- `{ "error": "..." }`
	- 주요 메시지 예시:
	  - `unauthorized`
	  - `invalid credentials`
	  - `bearer token is required`
	  - `invalid token`
	  - `insufficient permissions`
	  - `from must use RFC3339 format`
	  - `to must use RFC3339 format`
- 페이지네이션 응답
	- `{ "items": [...], "next_cursor": "..." }`
	- `next_cursor`는 다음 페이지가 없으면 생략됩니다.
- 성공 응답(대표)
	- 생성: `{ "id": 123 }`
	- 단순 성공: `{ "ok": true }`
	- URL 반환: `{ "url": "https://..." }`
	- 로그인: `{ "access_token": "...", "user_id": "...", "tenant_id": "default", "role": "admin" }`

## API 사용 예시(curl + 응답)

1) 로그인 (Public)

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
	-H "Content-Type: application/json" \
	-d '{"tenant_id":"default","email":"admin@example.com","password":"pass1234"}'
```

```json
{
	"access_token": "<JWT>",
	"user_id": "<user_id>",
	"tenant_id": "default",
	"role": "admin"
}
```

2) 채팅방 생성

```bash
curl -X POST http://localhost:8080/api/v1/rooms \
	-H "Authorization: Bearer <token>" \
	-H "Content-Type: application/json" \
	-d '{"name":"backend-team","room_type":"group","member_ids":[2,3]}'
```

```json
{
	"id": 101
}
```

3) 채팅방 목록(커서 페이지네이션)

```bash
curl -X GET "http://localhost:8080/api/v1/rooms?limit=2" \
	-H "Authorization: Bearer <token>"
```

```json
{
	"items": [
		{ "id": 101, "name": "backend-team" },
		{ "id": 100, "name": "ops" }
	],
	"next_cursor": "ey4uLg"
}
```

4) 별칭 감사 로그 조회(필터 + 커서)

```bash
curl -X GET "http://localhost:8080/api/v1/users/me/aliases/audit?limit=2&action=add" \
	-H "Authorization: Bearer <token>"
```

```json
{
	"items": [
		{
			"id": 10,
			"alias": "chul",
			"action": "add",
			"acted_by": 1,
			"ip": "127.0.0.1",
			"user_agent": "curl/8.5.0",
			"created_at": "2026-02-15T09:00:00Z"
		}
	],
	"next_cursor": "ey4uLg"
}
```

5) 방 안읽음 수 조회

```bash
curl -X GET http://localhost:8080/api/v1/rooms/101/unread-count \
	-H "Authorization: Bearer <token>"
```

```json
{
	"room_id": 101,
	"user_id": 1,
	"unread_count": 3
}
```

6) 파일 다운로드 Presign URL 발급

```bash
curl -X POST http://localhost:8080/api/v1/files/presign-download \
	-H "Authorization: Bearer <token>" \
	-H "Content-Type: application/json" \
	-d '{"object_key":"rooms/101/image.png"}'
```

```json
{
	"url": "https://minio.example/..."
}
```

- 조직도
	- `POST /org-units`
	- `GET /org-units`
- 테넌트 관리(admin/manager)
	- `GET /tenants`
	- `POST /tenants`
	- `PATCH /tenants/:id`
  - `deployment_mode`: `shared | dedicated`
  - `deployment_mode=dedicated`면 `dedicated_dsn` 필수
  - 전용 인프라 옵션(선택): `dedicated_redis_addr`, `dedicated_lavinmq_url`, `dedicated_minio_*`
- 사용자
	- `POST /users`
	- `PATCH /users/:id/status`
	- `GET /users/search?q=...&limit=20`
	- `GET /users/me/aliases`
	- `POST /users/me/aliases`
	- `DELETE /users/me/aliases`
	- `GET /users/me/aliases/audit?limit=50&action=add|delete&from=RFC3339&to=RFC3339&cursor=...`
	  - alias 규칙: 1~40자, `영문/숫자/한글/_`만 허용
	  - 저장 시 소문자 정규화, 대소문자 구분 없이 중복 불가
	  - audit 응답 필드: `action`, `acted_by`, `ip`, `user_agent`, `created_at`
	  - 페이지네이션 응답: `{ "items": [...], "next_cursor": "..." }`
- 채팅방
	- `GET /rooms?limit=50&cursor=...`
	  - 페이지네이션 응답: `{ "items": [...], "next_cursor": "..." }`
	- `POST /rooms`
	- `POST /rooms/:id/members`
	  - `room_type=direct` 인 경우 응답에 `peer_user_id`, `peer_name`, `peer_status`, `peer_status_note` 포함
	  - 최근 메시지 요약 필드: `latest_message_kind(text|file|emoji)`, `latest_message_summary`, `latest_message_mention_tokens`, `latest_message_is_mentioned`
	  - `latest_message_is_mentioned`는 사용자 `name`, 이메일 아이디, `user_aliases.alias` 기준으로 계산
- 메시지
	- `POST /rooms/:id/messages`
	- `GET /rooms/:id/messages?limit=50&cursor=...`
	  - 페이지네이션 응답: `{ "items": [...], "next_cursor": "..." }`
	- `GET /rooms/:id/unread-count`
	- `GET /rooms/unread-counts`
	- `POST /rooms/:id/read`
	- `GET /rooms/:id/read`
	- `GET /rooms/:id/messages/:messageId/readers`
	- `GET /messages/search?q=...&room_id=...&limit=30&cursor=...`
	  - 페이지네이션 응답: `{ "items": [...], "next_cursor": "..." }`
- 파일
	- `POST /files/presign-upload`
	- `POST /files/presign-download`
	- `POST /files/register`

WebSocket:
- `GET /ws?room_id={id}&access_token={jwt}`
	- 또는 `Authorization: Bearer <jwt>` 헤더 사용 가능
	- 서버에서 토큰 검증 + 방 멤버십 검증 후 연결 허용
	- `type=message` 이벤트는 WS 수신 시 DB에 즉시 저장 후 fan-out
- `payload.client_msg_id`를 함께 보내면 중복 전송 시 DB 중복 저장을 방지
- 클라이언트 JSON 메시지 타입 예:
	- 일반 채팅 이벤트: `{ "type": "message", "payload": {"client_msg_id":"...","body":"...","file_id":null,"file_ids":["f1","f2"],"emojis":[]} }`
	- WebRTC 시그널: `webrtc_offer`, `webrtc_answer`, `webrtc_ice`

브라우저 최소 예제(로그인 → 방 생성 → WS 전송):

```javascript
const baseUrl = "http://localhost:8080";

// 1) 로그인
const loginRes = await fetch(`${baseUrl}/api/v1/auth/login`, {
	method: "POST",
	headers: { "Content-Type": "application/json" },
	body: JSON.stringify({
		tenant_id: "default",
		email: "admin@example.com",
		password: "pass1234",
	}),
});
const { access_token } = await loginRes.json();

// 2) 방 생성
const roomRes = await fetch(`${baseUrl}/api/v1/rooms`, {
	method: "POST",
	headers: {
		"Content-Type": "application/json",
		Authorization: `Bearer ${access_token}`,
	},
	body: JSON.stringify({
		name: "ws-demo-room",
		room_type: "group",
		member_ids: [],
	}),
});
const { id: roomId } = await roomRes.json();

// 3) WS 연결 (브라우저에선 쿼리 토큰 방식이 간단)
const wsUrl = `ws://localhost:8080/ws?room_id=${encodeURIComponent(roomId)}&access_token=${encodeURIComponent(access_token)}`;
const ws = new WebSocket(wsUrl);

ws.onopen = () => {
	// 4) 실시간 이벤트 전송 (DB 저장 + Redis fan-out)
	ws.send(JSON.stringify({
		type: "message",
		payload: {
			client_msg_id: crypto.randomUUID(),
			body: "hello realtime with files",
			file_ids: ["file-id-1", "file-id-2"],
			emojis: [],
		},
	}));
};

ws.onmessage = (evt) => {
	console.log("ws message:", evt.data);
};
```

멀티파일 업로드 + 메시지 전송 유틸 예시(브라우저 JS):

```javascript
async function sendMessageWithFiles({
	baseUrl,
	token,
	roomId,
	tenantId = "default",
	body = "",
	files = [],
	ws,
	signal,
}) {
	const headers = {
		"Content-Type": "application/json",
		Authorization: `Bearer ${token}`,
	};

	const presigned = await Promise.allSettled(
		files.map(async (file) => {
			const res = await fetch(`${baseUrl}/api/v1/files/presign-upload`, {
				method: "POST",
				headers,
				signal,
				body: JSON.stringify({
					object_key: `rooms/${roomId}/${crypto.randomUUID()}-${file.name}`,
					content_type: file.type || "application/octet-stream",
				}),
			});
			if (!res.ok) throw new Error(`presign failed: ${file.name}`);
			const data = await res.json();
			return { file, objectKey: data.object_key, url: data.url };
		})
	);

	const uploaded = await Promise.allSettled(
		presigned
			.filter((r) => r.status === "fulfilled")
			.map((r) => r.value)
			.map(async (item) => {
				const put = await fetch(item.url, {
					method: "PUT",
					headers: { "Content-Type": item.file.type || "application/octet-stream" },
					signal,
					body: item.file,
				});
				if (!put.ok) throw new Error(`upload failed: ${item.file.name}`);
				return item;
			})
	);

	const fileIds = [];
	for (const item of uploaded) {
		if (item.status !== "fulfilled") continue;
		const res = await fetch(`${baseUrl}/api/v1/files/register`, {
			method: "POST",
			headers,
			signal,
			body: JSON.stringify({
				tenant_id: tenantId,
				room_id: roomId,
				object_key: item.value.objectKey,
				file_name: item.value.file.name,
				content_type: item.value.file.type || "application/octet-stream",
				size: item.value.file.size,
			}),
		});
		if (!res.ok) continue;
		const data = await res.json();
		fileIds.push(data.id);
	}

	if (!body.trim() && fileIds.length === 0) {
		throw new Error("no content to send");
	}

	const payload = {
		type: "message",
		payload: {
			client_msg_id: crypto.randomUUID(),
			body,
			file_ids: fileIds,
			emojis: [],
		},
	};

	// WS가 연결되어 있으면 실시간 전송, 아니면 REST fallback
	if (ws && ws.readyState === WebSocket.OPEN) {
		ws.send(JSON.stringify(payload));
		return { mode: "ws", file_ids: fileIds };
	}

	const sendRes = await fetch(`${baseUrl}/api/v1/rooms/${roomId}/messages`, {
		method: "POST",
		headers,
		signal,
		body: JSON.stringify({ body, file_ids: fileIds, emojis: [] }),
	});
	if (!sendRes.ok) throw new Error("send failed");
	return { mode: "rest", ...(await sendRes.json()) };
}

// 취소 예시
const controller = new AbortController();

sendMessageWithFiles({
	baseUrl: "http://localhost:8080",
	token,
	roomId,
	body: "hello",
	files,
	ws,
	signal: controller.signal,
}).catch((err) => {
	if (err.name === "AbortError") {
		console.log("upload/send canceled");
	}
});

// 사용자가 취소 버튼 클릭 시
// controller.abort();
```

업로드 진행률 표시가 필요한 경우(XHR 기반 PUT 예시):

```javascript
function putWithProgress(url, file, { signal, onProgress } = {}) {
	return new Promise((resolve, reject) => {
		const xhr = new XMLHttpRequest();
		xhr.open("PUT", url, true);
		xhr.setRequestHeader("Content-Type", file.type || "application/octet-stream");

		xhr.upload.onprogress = (evt) => {
			if (!evt.lengthComputable) return;
			const percent = Math.round((evt.loaded / evt.total) * 100);
			onProgress?.(percent, evt.loaded, evt.total);
		};

		xhr.onload = () => {
			if (xhr.status >= 200 && xhr.status < 300) {
				resolve();
				return;
			}
			reject(new Error(`upload failed: ${xhr.status}`));
		};

		xhr.onerror = () => reject(new Error("network error"));
		xhr.onabort = () => reject(new DOMException("Aborted", "AbortError"));

		if (signal) {
			if (signal.aborted) {
				xhr.abort();
				return;
			}
			signal.addEventListener("abort", () => xhr.abort(), { once: true });
		}

		xhr.send(file);
	});
}

// 사용 예시: presigned URL로 업로드하면서 진행률 갱신
await putWithProgress(presignedUrl, file, {
	signal: controller.signal,
	onProgress: (percent) => {
		console.log(`upload ${file.name}: ${percent}%`);
	},
});
```

Postman 컬렉션:
- 컬렉션: `postman/msg_server.postman_collection.json`
	- 스모크 컬렉션: `postman/msg_server.smoke.postman_collection.json`
- 환경:
	- `postman/msg_server.local.postman_environment.json`
	- `postman/msg_server.staging.postman_environment.json`
- 사용 순서:
	1. 컬렉션 + 환경 파일 Import
	2. 활성 환경 선택(local 또는 staging)
	3. `POST /api/v1/auth/login` 실행(토큰 자동 저장)

Newman 스모크 테스트:
- `make smoke`
- 또는 `bash ./scripts/newman_smoke.sh`
- 스테이징 환경 지정: `bash ./scripts/newman_smoke.sh ./postman/msg_server.staging.postman_environment.json`
- 오버라이드 변수: `SMOKE_BASE_URL`, `SMOKE_TENANT_ID`, `SMOKE_EMAIL`, `SMOKE_PASSWORD`

GitHub Actions 스모크 CI:
- 워크플로우: `.github/workflows/smoke.yml`
- 흐름: 인프라 기동 → 마이그레이션 → 관리자 시드 → 서버 실행 → `make smoke`
- 스모크 로그는 `happy-path` / `error-contract` 섹션으로 그룹 출력됩니다.
- `workflow_dispatch` 입력: `tenant_id`, `admin_email`, `base_url`, `run_dbman_failover_smoke`
	- 예시: `tenant_id=default`, `admin_email=admin@example.com`, `base_url=http://localhost:8080`, `run_dbman_failover_smoke=false`
- 비밀번호 소스: `secrets.SMOKE_ADMIN_PASSWORD` (필수)
- 실행 시 비밀번호 소스를 로그로 안내하고, 시크릿이 없으면 즉시 실패합니다.
- `pull_request`, `push(main)`, `workflow_dispatch` 모든 이벤트에서 동일하게 시크릿 필수 정책을 사용합니다.

workflow_dispatch 빠른 실행 체크리스트:
1. `Actions` → `smoke` → `Run workflow`에서 `tenant_id`, `admin_email`, `base_url`을 입력합니다.
2. 필요 시 `run_dbman_failover_smoke=true`로 선택 실행합니다.
3. `SMOKE_ADMIN_PASSWORD` 시크릿 존재 여부를 확인합니다.
4. 실패 시 `Debug tenant endpoint on smoke failure` 단계 출력을 확인합니다.
	- 로그인 실패(401): 아래 `트러블슈팅(로그인 401)` 절차 확인
	- 헬스체크 실패: `Wait for infra health`, `Wait for API health`, `Print API log on failure` 순서로 로그 확인

workflow_dispatch 입력 템플릿(복붙용):

```text
tenant_id=default
# 멀티테넌트 검증 시: tenant_id=tenant-demo
admin_email=admin@example.com
base_url=http://localhost:8080
```

- 사전조건: `tenant_id=tenant-demo` 사용 시 해당 테넌트에 `admin_email` 계정이 시드되어 있어야 합니다.
- 준비 예시: `make seed-admin TENANT_ID=tenant-demo ADMIN_EMAIL=admin@example.com ADMIN_PASSWORD='<pw>' ADMIN_NAME='Tenant Admin' ADMIN_TITLE='Administrator'`
- tenant-demo 실행 순서:
	1. 위 `make seed-admin ...` 명령으로 `tenant-demo` 관리자 계정 시드
	2. `Actions > smoke > Run workflow`에서 `tenant_id=tenant-demo`, `admin_email=admin@example.com`로 실행

GitHub Actions 시크릿 체크리스트:
- 경로: `Repository Settings` → `Secrets and variables` → `Actions`
- 필수 시크릿:
	- `SMOKE_ADMIN_PASSWORD`: 스모크 로그인에 사용할 관리자 비밀번호
- 권장 설정:
	- 짧은 테스트 전용 비밀번호 대신 충분히 강한 랜덤 문자열 사용
	- 주기적 로테이션 및 유출 의심 시 즉시 재발급
	- 시크릿 값은 워크플로우 로그/문서/코드에 직접 출력하지 않기

트러블슈팅(시크릿 미설정):
- 증상(로그 예시):
	- `::error::smoke workflow requires repository secret SMOKE_ADMIN_PASSWORD.`
	- `Error: Process completed with exit code 1.`
- 조치:
	1. `Repository Settings` → `Secrets and variables` → `Actions`
	2. `New repository secret`에서 `SMOKE_ADMIN_PASSWORD` 생성
	3. 워크플로우 재실행 (`Re-run jobs` 또는 `Run workflow`)

트러블슈팅(로그인 401):
- 증상(로그 예시):
	- `POST /api/v1/auth/login` 요청이 `401 Unauthorized`
	- Newman assertion 실패 (예: `expected response to have status code 200 but got 401`)
- 주요 원인:
	- `SMOKE_ADMIN_PASSWORD` 값과 시드 단계의 비밀번호 불일치
	- `tenant_id` 입력값과 시드 대상 테넌트 불일치
	- 시드 단계 실패로 관리자 계정이 생성/갱신되지 않음
	- `admin_email` 입력값과 시드 대상 이메일 불일치
- 조치:
	1. 워크플로우 로그에서 `Seed admin user` 단계 성공 여부를 확인합니다.
	2. `SMOKE_ADMIN_PASSWORD` 값을 갱신한 뒤 워크플로우를 재실행합니다.
	3. `workflow_dispatch` 실행 시 `tenant_id`, `admin_email`을 시드 계정과 동일하게 설정합니다.
	4. `tenant-demo`를 사용하는 경우 상단의 `tenant-demo 실행 순서`(시드 → workflow_dispatch)를 그대로 수행합니다.

트러블슈팅(API health timeout):
- 증상(로그 예시):
	- `api not ready in time`
	- `Wait for API health` 단계 실패
- 주요 원인:
	- 인프라 컨테이너 중 일부가 healthy 상태가 아님
	- 서버 기동 실패(환경변수 오타, 포트 충돌, 외부 의존성 연결 실패)
	- DB 마이그레이션/시드 실패로 서버 초기화가 중단됨
- 조치:
	1. `Wait for infra health` 단계 로그에서 `postgres/redis/lavinmq/minio` 상태를 확인합니다.
	2. 실패 시 `Print API log on failure` 출력에서 서버 에러 원인을 확인합니다.
	3. `POSTGRES_DSN`, `REDIS_ADDR`, `LAVINMQ_URL`, `MINIO_*` 값과 포트를 점검한 뒤 재실행합니다.

빠른 진단 명령(1-liner):

```bash
# 1) 로그인 401 즉시 확인
curl -s -o /tmp/login.json -w "%{http_code}\n" -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d "{\"tenant_id\":\"${SMOKE_TENANT_ID:-default}\",\"email\":\"admin@example.com\",\"password\":\"$SMOKE_ADMIN_PASSWORD\"}" && cat /tmp/login.json

# 2) 시드 관리자 계정 존재/권한 확인
psql "$POSTGRES_DSN" -c "select user_id,email,role,status,updated_at from users where email='admin@example.com';"

# 3) API 헬스 응답 확인
curl -i http://localhost:8080/health

# 4) 서버 로그 확인(로컬 실행 시)
tail -n 200 /tmp/msg_server.log

# 5) 인프라 컨테이너 상태 확인
docker compose ps
```

진단 스크립트(권장):
- `make diag`
- `make diag-report` (결과를 `./diag_report.txt`에 저장)
- 또는 `bash ./scripts/quick_diag.sh`
- dbman failover 스모크: `make dbman-failover-smoke`
- 또는 `bash ./scripts/dbman_failover_smoke.sh`
- MQ 비활성(WS fan-out 전용) 스모크: `CHAT_USE_MQ=false make run-chat` 후 `make chat-ws-only-smoke`
- 또는 `BASE_URL=http://localhost:8080 TENANT_ID=default SMOKE_EMAIL=admin@example.com SMOKE_PASSWORD='<pw>' bash ./scripts/chat_ws_only_smoke.sh`
- k6 설치(Ubuntu/dev container): `make install-k6`
- k6 기준선 부하 테스트: `make load-chat-baseline`
- k6 커스텀 부하 테스트: `K6_VUS=2000 K6_DURATION=15m BASE_URL=http://localhost:8080 TENANT_ID=default SMOKE_EMAIL=admin@example.com SMOKE_PASSWORD='<pw>' make load-chat`
- k6 리포트 저장+요약: `K6_VUS=2000 K6_DURATION=15m REPORT_DIR=./loadtest_reports make load-chat-report`
  - 생성 파일: `k6_chat_hotpath_summary_<UTC>.json`, `k6_chat_hotpath_console_<UTC>.log`
- 리포트 파일 저장: `DIAG_REPORT=./diag_report.txt bash ./scripts/quick_diag.sh`
- 옵션 환경변수:
	- `BASE_URL` (기본: `http://localhost:8080`)
	- `ADMIN_EMAIL` (기본: `admin@example.com`)
	- `SMOKE_ADMIN_PASSWORD` (로그인 점검용)
	- `SMOKE_EMAIL`, `SMOKE_PASSWORD` (ws-only smoke 로그인용)
	- `K6_VUS`, `K6_DURATION`, `K6_SLEEP_MS` (k6 부하 테스트 파라미터)
	- `REPORT_DIR` (k6 리포트 출력 경로)
	- `POSTGRES_DSN` (DB 조회용)
	- `DIAG_REPORT` (진단 결과 파일 경로)
	- `GOOD_DBMAN_ENDPOINT`, `BAD_DBMAN_ENDPOINT` (failover 스모크용 endpoint 오버라이드)

## 디렉터리

- `cmd/chat`: API 서버 엔트리포인트
- `cmd/session`: 세션/알림 서버 엔트리포인트
- `cmd/fileman`: 파일 전용 서버 엔트리포인트
- `cmd/dbman`: DB DML/검색 전용 서버 엔트리포인트
- `cmd/vectorman`: 벡터 DML/검색 전용 서버 엔트리포인트
- `server/common`: 서비스 공통 미들웨어 및 인프라(db, cache, mq, object)
- `server/chat`: 메인 채팅 서버 코드(API, 서비스, 저장소, 인프라)
- `server/session`: 세션/알림 서버 코드(api/app/domain/repository/service)
- `server/fileman`: 파일 전용 서버 코드(api/app)
- `server/dbman`: DB DML/조회 전용 서버 코드(api/app/domain/repository)
- `server/vectorman`: Milvus 벡터 DML/검색 전용 서버 코드(api/app/service)
- `migrations`: SQL 스키마
	- `004_user_aliases.sql`: 멘션 별칭(alias) 테이블
	- `006_alias_audit.sql`: alias 추가/삭제 감사 로그 테이블
	- `007_alias_audit_meta.sql`: 감사 로그에 IP/User-Agent 컬럼 추가
- `scripts/migrate.sh`: 마이그레이션 실행 스크립트

## 다음 확장 권장

- 인증/인가(JWT, 조직별 RBAC)
- 메시지 전달보장(consumer, retry, DLQ)
- 읽음/안읽음, 멘션, 고정메시지, 스레드
- 실제 Milvus 컬렉션 생성/스키마/인덱스 자동화
- SFU 기반 음성/화상 통화 서버 연동

## 권한 정책(MVP)

- `admin`, `manager`: 조직/사용자 생성, 조직 조회
- 모든 보호 API는 JWT 필요
- 상태 변경은 본인 또는 `admin`/`manager`만 가능