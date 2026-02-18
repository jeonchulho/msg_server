# 10만 사용자 대비 부하 테스트/운영 계획

본 문서는 현재 `msg_server` 구조를 기준으로 10만 사용자 규모를 목표로 할 때의 실행 계획을 정리합니다.

전제:
- 단일 리전, 다중 인스턴스(Kubernetes) 배포
- `chat -> dbman -> postgres` 경로를 유지
- 10만은 MAU가 아닌 **피크 동시접속(CCU) 검증 관점**으로 가정

## 목표 SLO (권장 초안)

- 메시지 저장(WS/REST) 성공률: 99.9% 이상
- 메시지 저장 p95: 300ms 이하, p99: 800ms 이하
- 메시지 조회 p95: 400ms 이하
- 장애 시 복구(단일 인스턴스 다운): 5분 이내

## 병목 우선순위

1. P1: 메시지 저장 경로
   - WS: `server/chat/service/realtime_service.go` → `chat.CreateMessage`
   - REST: `POST /api/v1/rooms/:id/messages`
2. P1: dbman API/DB write 처리량
3. P2: 메시지 조회/검색 (`/messages/list`, `/messages/search`)
4. P2: 테넌트 인프라 메타 조회 캐시 miss 경로

## 단계별 검증 시나리오

### Stage 0: 기준선(Baseline)
- 목표: 기능/로그/지표 정상 수집 확인
- 부하: 200~500 VUs, 10분
- 통과: 에러율 < 1%, p95 < 500ms

### Stage 1: 중간 부하
- 목표: 병목 지점 식별
- 부하: 2,000~5,000 VUs, 20분
- 통과: 에러율 < 1%, p95 < 400ms

### Stage 2: 고부하
- 목표: 스케일 아웃 정책 검증(HPA/자원 한계)
- 부하: 10,000~20,000 VUs, 20~30분
- 통과: 에러율 < 1.5%, p95 < 400ms, p99 < 1s

### Stage 3: 목표 검증
- 목표: 10만 사용자 운영 가능성 평가
- 부하: 시나리오 분할로 합산 50,000~100,000 동시 사용자
  - 예: 다중 워커/노드에서 동일 스크립트 병렬 실행
- 통과: 목표 SLO 달성 + 장애 주입 테스트 통과

## 실행 방법 (k6)

사전 준비:

```bash
make install-k6
```

기본 실행:

```bash
make load-chat-baseline
```

고부하 예시:

```bash
BASE_URL=http://localhost:8080 \
TENANT_ID=default \
SMOKE_EMAIL=admin@example.com \
SMOKE_PASSWORD=pass1234 \
K6_VUS=5000 \
K6_DURATION=20m \
make load-chat
```

## 장애 주입 체크

- `chat` 인스턴스 1개 강제 종료 시 에러율 급등/회복 시간
- `dbman` 인스턴스 1개 강제 종료 시 failover 동작
- Redis 일시 지연/네트워크 제한 시 WS fan-out 품질

## 관측 항목 (최소)

- `event=chat_message_persist` 로그
  - `source=rest|ws`, `status=ok|failed`, `latency_ms`
- `dbman` 요청 실패율/타임아웃 비율
- PostgreSQL: TPS, lock wait, slow query, connection usage
- Redis: publish latency, dropped connection, CPU/memory

## 운영 전 체크리스트

- [ ] `dbman`, `chat`, `session` HPA 기준 정의
- [ ] PostgreSQL 커넥션 풀 상한/쿼리 인덱스 점검
- [ ] Redis 클러스터/샤딩 계획
- [ ] 알림 임계치(SLO breach) 설정
- [ ] 주간 정기 부하 테스트 파이프라인 구성
