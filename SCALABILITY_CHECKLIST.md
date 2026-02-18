# 대용량 준비도 체크리스트

본 문서는 현재 코드/문서 상태를 기준으로 대용량 운영 준비도를 빠르게 점검하기 위한 체크리스트입니다.

가정:
- 단일 리전, 다중 인스턴스 배포를 전제로 함
- 고트래픽(피크 동시접속/고QPS)에서 안정성 확보가 목표

## 요약 진단

- 현재 구조는 **MVP~중간 트래픽**에는 적합
- 대용량 운영 관점에서는 **핵심 병목 지점(dbman 집중 경로, 동기 호출 체인)** 보완 필요

상세 실행 문서:
- [DBMAN_HA_TODO.md](DBMAN_HA_TODO.md)
- [LOAD_TEST_100K_PLAN.md](LOAD_TEST_100K_PLAN.md)

## 항목별 점검

| 영역 | 현재 상태 | 근거 | 우선 액션 |
|---|---|---|---|
| 서비스 경계/분리 | 양호 | chat/session/fileman/dbman/vectorman 분리 | 서비스별 독립 스케일 정책 문서화 |
| DB 접근 일관성 | 양호 | DB DML/조회가 dbman으로 집중 | dbman 수평 확장 + 읽기/쓰기 분리 계획 |
| 단일 장애 지점(SPOF) | 보완 필요 | 다수 서비스가 dbman endpoint 경유 | dbman 다중 인스턴스 + LB/헬스체크 + 서킷브레이커 |
| 동기 호출 체인 지연 | 보완 필요 | service→dbman HTTP 동기 경로 존재 | 타임아웃/재시도 정책 표준화, 비동기화(outbox) 도입 |
| 캐시/무효화 전략 | 부분 구현 | tenant 라우터 캐시/무효화 존재 | room/member/unread hot-path 캐시 확장 |
| 메시지 전달 내구성 | 부분 구현 | MQ 사용 기반 존재 | 재시도/DLQ/idempotency key 강화 |
| 검색/벡터 처리 | 부분 구현 | vectorman 위임 + Milvus 연동 골격 | 색인 실패 재처리 큐, 배치/백프레셔 추가 |
| 파일 처리 확장성 | 부분 구현 | fileman 분리 + MinIO 라우팅 | 썸네일 작업 비동기 워커 전환 검토 |
| 관측성(Observability) | 보완 필요 | 기본 헬스체크 중심 | OTel trace, RED/USE 메트릭, 상관관계 ID 도입 |
| 부하/장애 테스트 | 보완 필요 | 스모크 테스트 중심 | k6 시나리오 + 장애주입 테스트 CI화 |
| 보안/운영 거버넌스 | 부분 구현 | JWT 기반 인증/권한 정책 존재 | 시크릿 로테이션/감사/레이트리밋 강화 |

## 1차 실행 계획 (권장 순서)

1. dbman 고가용성
   - 다중 인스턴스, LB, readiness/liveness 분리
   - DB 커넥션 풀/쿼리 상한값 튜닝
2. 동기 체인 안정화
   - 서비스 간 타임아웃/재시도/백오프 표준화
   - 실패 격리를 위한 서킷브레이커 적용
3. 이벤트 내구성 강화
   - outbox 패턴 + DLQ + 중복처리 방지 키 도입
4. 관측성 구축
   - Trace ID 전파, 지표 대시보드, 알림 임계치 정의
5. 성능 검증 자동화
   - k6 부하 테스트, 장애주입 테스트를 CI/주간 배치화

## 체크 기준(통과 조건 예시)

- 피크 트래픽에서 p95 응답시간 목표 충족
- dbman 인스턴스 1대 장애 시에도 SLO 유지
- 메시지/이벤트 유실률 목표치 이하
- 장애 탐지→알림→복구 평균 시간(MTTR) 목표 충족

## 근거 파일

- [README.md](README.md)
- [server/dbman/app/server.go](server/dbman/app/server.go)
- [server/chat/app/server.go](server/chat/app/server.go)
- [server/session/app/server.go](server/session/app/server.go)
- [server/fileman/service/file_service.go](server/fileman/service/file_service.go)
- [server/common/infra/dbman/client.go](server/common/infra/dbman/client.go)
- [server/vectorman/service/milvus_service.go](server/vectorman/service/milvus_service.go)
