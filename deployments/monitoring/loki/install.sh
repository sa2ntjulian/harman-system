#!/bin/bash
set -e

echo "Loki 배포 중..."

helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -

helm upgrade --install loki grafana/loki \
  --namespace monitoring \
  --values values.yaml \
  --wait

echo "Loki 배포 완료"

