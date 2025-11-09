# Harman Kubernetes 파일 수집 시스템

Kubernetes DaemonSet을 이용하여 각 노드의 특정 디렉토리 파일 목록을 주기적으로 수집하고 MySQL 데이터베이스에 저장하는 시스템입니다.

## 배포 방법

### 1. MySQL 배포

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

kubectl create namespace harman-system

cd deployments/mysql
helm dependency update
helm install mysql . -n harman-system

kubectl wait --for=condition=ready pod mysql-0 -n harman-system --timeout=300s
```

### 2. 애플리케이션 이미지 빌드

```bash
docker build -t harman-collector:latest .
minikube image load harman-collector:latest -p harman
```

### 3. DaemonSet 배포

```bash
kubectl apply -f deployments/daemonset/namespace.yaml
kubectl apply -f deployments/daemonset/secret.yaml
kubectl apply -f deployments/daemonset/configmap.yaml
kubectl apply -f deployments/daemonset/daemonset.yaml

kubectl get daemonset -n harman-system
kubectl get pods -n harman-system -l app=file-collector
```

## 데이터 확인

```bash
MYSQL_POD=$(kubectl get pods -n harman-system -l app.kubernetes.io/name=mysql -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it -n harman-system $MYSQL_POD -- mysql -uharman -pharman123\!\@\# harman

SELECT * FROM file_collections ORDER BY collected_at DESC LIMIT 10;
SELECT * FROM collected_files LIMIT 20;
```

## 배포 자동화 (ArgoCD)

ArgoCD를 사용하여 GitOps 방식으로 배포를 자동화할 수 있습니다. Git 저장소의 변경사항이 자동으로 클러스터에 반영됩니다.

### ArgoCD 배포

```bash
./scripts/deploy-argocd.sh
```

### Application 생성

```bash
kubectl apply -f deployments/argocd/apps/collector-dev.yaml
kubectl apply -f deployments/argocd/apps/mysql-dev.yaml
```

Kustomize를 사용하여 dev/prod 환경별로 리소스를 관리할 수 있습니다:

```bash
# 개발 환경 배포
kubectl apply -k deployments/kustomize/overlays/dev

# 프로덕션 환경 배포
kubectl apply -k deployments/kustomize/overlays/prod
```

## 모니터링

OpenTelemetry, Prometheus, Grafana를 사용하여 애플리케이션의 메트릭, 트레이스, 로그를 수집하고 시각화할 수 있습니다.

### 모니터링 스택 배포

```bash
./scripts/deploy-monitoring.sh
```

### Grafana 접근

```bash
minikube service -n monitoring prometheus-grafana --url
```

Grafana에서 File Collector의 메트릭과 트레이스를 확인할 수 있습니다.

## 환경 변수

- `NODE_NAME`: Kubernetes 노드명 (자동 주입)
- `MOUNT_PATH`: 스캔할 디렉토리 경로 (기본값: `/mnt/harman`)
- `COLLECT_INTERVAL_MINUTES`: 수집 주기 (분, 기본값: `1`)
- `DB_HOST`: MySQL 호스트 (기본값: `mysql.harman-system.svc.cluster.local`)
- `DB_PORT`: MySQL 포트 (기본값: `3306`)
- `DB_USER`: MySQL 사용자명 (기본값: `harman`)
- `DB_PASSWORD`: MySQL 비밀번호
- `DB_NAME`: 데이터베이스명 (기본값: `harman`)
- `LOG_LEVEL`: 로그 레벨 (기본값: `info`)