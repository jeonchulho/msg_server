# Release Notes

## 2026-02-15

### 범위
- fileman object key prefix 결합 로직 정규화
- vectorman Milvus endpoint/path 결합 정규화
- common dbman HTTP client path 결합 정규화
- README 서비스 경계/디렉터리 설명 문구 정확화

### 변경 상세
- `server/fileman/service/file_service.go`
  - `tenantObjectKey`에서 prefix를 정규화(앞 슬래시 제거, 끝 슬래시 보정, 중복 prefix 체크 기준 통일)
- `server/vectorman/service/milvus_service.go`
  - `MILVUS_ENDPOINT` 정규화(`TrimSpace`, trailing slash 제거)
  - 내부 `post` 호출 path가 `/`로 시작하지 않으면 자동 보정
- `server/common/infra/dbman/client.go`
  - `Post` 호출 path가 `/`로 시작하지 않으면 자동 보정
- `README.md`
  - 현재 아키텍처 기준으로 디렉터리 설명 문구 보정

### 운영 영향
- API 스펙/엔드포인트 변경 없음
- DB 스키마/마이그레이션 변경 없음
- 기존 환경변수 유지
- 경계 케이스(슬래시 유무)에 대한 안정성 향상

### 검증
- 전체 빌드 검증 완료: `go build ./...` → `BUILD_OK`

### 배포/롤백
- 무중단 반영 가능(호환성 영향 없음)
- 문제 발생 시 본 릴리즈 커밋 단위 롤백 권장
