#!/bin/bash
set -e

echo "OpenTelemetry Collector 배포 중..."

helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update

kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -

helm upgrade --install opentelemetry-collector open-telemetry/opentelemetry-collector \
  --namespace monitoring \
  --values values.yaml \
  --wait

echo "OpenTelemetry Collector 배포 완료"

