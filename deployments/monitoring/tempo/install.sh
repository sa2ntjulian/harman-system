#!/bin/bash
set -e

echo "Tempo 배포 중..."

helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -

helm upgrade --install tempo grafana/tempo \
  --namespace monitoring \
  --values values.yaml \
  --wait

echo "Tempo 배포 완료"

