# DBMan 고가용성 구현 TODO

이 문서는 dbman 단일 병목/단일 장애 지점 완화를 목표로, 바로 작업 가능한 단위로 쪼갠 실행 계획입니다.

## 목표

- dbman 다중 인스턴스 운영
- 인스턴스 일부 장애 시 서비스 연속성 유지
- 피크 트래픽에서도 응답 지연/실패율 제어

## 작업 단위 (파일 기준)

### 1) 환경변수/설정 확장

- 대상 파일: [cmd/chat/main.go](cmd/chat/main.go)
  - `DBMAN_ENDPOINTS`(comma-separated) 추가
  - 기존 `DBMAN_ENDPOINT`와 하위 호환 유지
- 대상 파일: [cmd/session/main.go](cmd/session/main.go)
  - 동일 정책 적용
- 대상 파일: [cmd/fileman/main.go](cmd/fileman/main.go)
  - 동일 정책 적용
- 대상 파일: [cmd/dbman/main.go](cmd/dbman/main.go)
  - 운영용 readiness/liveness 분리 포트 또는 플래그 추가 검토

완료 기준:
- 단일 endpoint 설정 없이도 기존 실행 가능
- 다중 endpoint 설정 시 파싱 오류 없이 기동

### 2) 공통 dbman 클라이언트 다중 엔드포인트 지원

- 대상 파일: [server/common/infra/dbman/client.go](server/common/infra/dbman/client.go)
  - `[]endpoint` 기반 생성자 추가
  - 요청마다 endpoint 선택(라운드로빈)
  - endpoint 장애 시 다음 endpoint로 failover(제한 재시도)
  - timeout/재시도/백오프 기본값 상수화

완료 기준:
- endpoint 1개 장애 시 요청 성공률 유지(다른 endpoint 정상 가정)
- path 정규화 규칙 기존과 동일 유지

### 3) 각 서비스 dbman 클라이언트 연결부 교체

- 대상 파일: [server/chat/service/dbman_client.go](server/chat/service/dbman_client.go)
- 대상 파일: [server/session/service/dbman_client.go](server/session/service/dbman_client.go)
- 대상 파일: [server/fileman/service/dbman_client.go](server/fileman/service/dbman_client.go)
  - 새 공통 생성자 사용
  - 실패 로그에 endpoint 정보 포함(민감정보 제외)

완료 기준:
- 기존 API 동작 변화 없음
- 빌드/기본 스모크 통과

### 4) dbman 서버 헬스체크 분리

- 대상 파일: [server/dbman/api/handler.go](server/dbman/api/handler.go)
  - liveness: 프로세스 생존 확인
  - readiness: DB 연결/핵심 의존성 확인
- 대상 파일: [server/dbman/app/server.go](server/dbman/app/server.go)
  - 라우트 등록 및 graceful shutdown 점검

완료 기준:
- readiness 실패 시 LB에서 트래픽 제외 가능
- liveness/readiness 의미가 분리됨

### 5) 런타임 보호장치

- 대상 파일: [server/common/infra/dbman/client.go](server/common/infra/dbman/client.go)
  - 간단 서킷브레이커(연속 실패 임계치) 또는 최소한의 실패 억제 로직
- 대상 파일: [README.md](README.md)
- 대상 파일: [.env.example](.env.example)
  - 신규 변수 문서화(`DBMAN_ENDPOINTS`, retry/timeout 관련)

완료 기준:
- 장애 전파 완화(폭주 재시도 방지)
- 운영자가 설정값 의미를 문서만으로 파악 가능

### 6) 검증 자동화

- 대상 파일: [scripts](scripts)
  - endpoint failover 검증 스크립트 추가
- 대상 파일: [.github/workflows](.github/workflows)
  - 선택 실행 가능한 HA smoke job 추가 검토

완료 기준:
- dbman 1개 노드 중단 시 핵심 API smoke 통과

## 권장 구현 순서

1. 설정 확장
2. 공통 클라이언트 다중 endpoint + failover
3. 서비스 연결부 교체
4. 헬스체크 분리
5. 보호장치/문서화
6. 자동화 검증

## 리스크 메모

- 과도한 재시도는 지연 급증을 만들 수 있으므로 총 시도 횟수 제한 필요
- failover 중 비멱등 요청의 중복 처리 가능성 점검 필요
- readiness 기준이 과도하면 flapping 가능성 있으므로 안정 임계치 필요
