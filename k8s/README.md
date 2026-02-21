# Kubernetes 배포 골격

이 디렉터리는 `msg_server`의 Kubernetes 배포 베이스 + 환경별 오버레이를 제공합니다.

## 포함 리소스

- `namespace.yaml`: 네임스페이스(`msg`)
- `configmap.yaml`: 비민감 환경변수
- `secret.example.yaml`: 시크릿 예시(실운영에서는 값 교체 필요)
- `chat.yaml`, `orghub.yaml`, `tenanthub.yaml`, `session.yaml`, `fileman.yaml`, `dbman.yaml`, `vectorman.yaml`
  - 각 서비스 Deployment + Service
- `pdb.yaml`
  - `dbman` PodDisruptionBudget (`minAvailable: 1`)
- `ingress.yaml`: chat/orghub/tenanthub/session/fileman 라우팅 예시
- `hpa.yaml`: 주요 서비스 HPA 예시
- `kustomization.yaml`: 베이스 리소스 묶음
- `base/`: 오버레이가 참조하는 베이스 kustomization 디렉터리
- `overlays/dev`, `overlays/staging`, `overlays/prod`
  - 환경별 `ConfigMap` 값, replicas, ingress host, image tag 패치
  - `staging/prod`는 `patch-dbman-hpa.yaml`로 `dbman` HPA(min/max/behavior/CPU+memory) 튜닝

## 적용 방법

환경별 오버레이 적용(권장):

```bash
kubectl apply -k ./k8s/overlays/dev
kubectl apply -k ./k8s/overlays/staging
kubectl apply -k ./k8s/overlays/prod
```

베이스만 적용(테스트/검증용):

```bash
kubectl apply -k ./k8s
```

## 사전 준비

- 이미지 태그를 실제 레지스트리/버전으로 교체
- `secret.example.yaml` 값을 실운영 값으로 교체(또는 SealedSecret/ExternalSecret 사용)
- PostgreSQL/Redis/LavinMQ/MinIO/Milvus 엔드포인트를 환경에 맞게 수정
- Ingress host를 실제 도메인으로 교체
