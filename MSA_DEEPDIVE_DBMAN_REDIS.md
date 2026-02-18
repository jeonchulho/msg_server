# MSA Deep Dive: dbman 확장 & Redis 채널 샤딩

본 문서는 [MSA_ROADMAP.md](MSA_ROADMAP.md)의 아래 두 항목을 실행 가능한 수준으로 상세화합니다.

1. `dbman` 수평 확장 + readiness/liveness + 오토스케일
2. Redis 채널 핫스팟 완화(샤딩/채널 전략)

---

## 1) dbman 수평 확장 + readiness/liveness + 오토스케일

### 1-1. 왜 중요한가

- 현재 `chat/session/fileman`은 DB 접근을 `dbman` API로 위임합니다.
- 즉 `dbman`은 공용 데이터 게이트웨이이며, 여기의 지연/장애가 전체로 전파됩니다.

관련 파일:
- [server/chat/app/server.go](server/chat/app/server.go)
- [server/session/app/server.go](server/session/app/server.go)
- [server/fileman/app/server.go](server/fileman/app/server.go)

### 1-2. health 체크 기준

현재 `dbman`은 이미 liveness/readiness를 분리해 사용 중입니다.

- liveness: `/health/live`
  - 프로세스 생존만 확인
  - 실패 시 컨테이너 재시작
- readiness: `/health/ready`
  - DB 접근 가능 여부까지 확인
  - 실패 시 서비스 트래픽에서 제외

관련 파일:
- [k8s/base/dbman.yaml](k8s/base/dbman.yaml)
- [server/dbman/api/handler.go](server/dbman/api/handler.go)

### 1-3. 수평 확장 체크리스트

- `dbman`을 무상태(stateless)로 유지
- 최소 replica 2 이상(운영은 3 이상 권장)
- Pod 분산 배치(anti-affinity)
- PDB 적용(업데이트/노드 이벤트 시 동시 축출 방지)
- 롤링 업데이트 전략 튜닝(`maxUnavailable` 최소화)

### 1-4. HPA 권장 기준

현재 HPA 리소스가 존재합니다.

관련 파일:
- [k8s/base/hpa.yaml](k8s/base/hpa.yaml)

권장 스케일 입력 지표:
- 1차: CPU + Memory
- 2차: 커스텀 지표(가능 시)
  - `dbman` 요청 p95 latency
  - `dbman` 5xx rate
  - timeout rate

운영 권장값(초기 예시):
- minReplicas: 3
- maxReplicas: 20~50 (클러스터 용량에 따라)
- scaleUp: 빠르게, scaleDown: 완만하게

### 1-5. DB 커넥션/쿼리 주의점

`dbman` replica 수만 늘리면 DB 연결 폭증이 발생할 수 있습니다.

동시 튜닝 필요:
- `dbman` 인스턴스당 DB 풀 상한
- PostgreSQL max connections
- 쿼리 인덱스/슬로우쿼리 관리

---

## 2) Redis 채널 핫스팟 완화(샤딩/채널 전략)

### 2-1. 현재 구조와 핫스팟 지점

현재 실시간 fan-out 채널은 room 단위입니다.

- 채널 예: `tenant:{tenant_id}:room:{room_id}`

관련 파일:
- [server/chat/service/realtime_service.go](server/chat/service/realtime_service.go)

리스크:
- 대형 방(핫룸)에 publish/subscribe 집중
- 특정 채널에서 지연 급증

### 2-2. 샤딩 전략

채널 키를 shard 단위로 확장합니다.

- 제안 키: `tenant:{tenant_id}:room:{room_id}:shard:{n}`
- shard 계산: `hash(message_id or sender_id) % N`
- N은 room별 트래픽 기반으로 동적 설정(초기 4~8)

장점:
- 단일 채널 집중 완화
- Redis 코어/네트워크 분산

주의:
- 방 전체 strict ordering이 약해질 수 있음
- 순서가 중요한 이벤트는 sequence 부여 후 클라이언트 재정렬 필요

### 2-3. 채널 분리 전략

이벤트 타입별 채널 분리로 간섭을 줄입니다.

- 메시지: `...:message:shard:{n}`
- 시그널링: `...:signal`
- 시스템 이벤트: `...:system`

효과:
- 시그널링 품질 보호
- 고빈도 메시지로 인한 전체 지연 전파 감소

### 2-4. 단계적 마이그레이션(권장)

1) Shadow publish
- 기존 채널 + 샤드 채널 동시 publish
- 소비는 기존 채널 유지

2) Canary subscribe
- 일부 tenant/room만 샤드 subscribe 전환

3) Full cutover
- 안정화 후 기존 단일 채널 폐기

4) 정리
- 모니터링 임계치 재조정
- runbook 갱신

### 2-5. 모니터링 항목

- 채널별 publish latency
- 채널별 subscriber 수
- slow consumer 세션 수
- 메시지 드롭/재전송 비율

---

## 실행 우선순위 (권장)

1. `dbman` HPA + PDB + anti-affinity 정비
2. `dbman` p95/5xx 대시보드 우선 구축
3. Redis 채널 타입 분리(메시지/시그널링)
4. room 샤딩을 canary로 도입
5. 부하테스트(k6) + 장애주입으로 승인
