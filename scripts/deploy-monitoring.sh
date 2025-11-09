#!/bin/bash
set -e

echo "모니터링 스택 배포 시작..."

kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -

echo "1. OpenTelemetry Collector 배포 중..."
cd deployments/monitoring/otel-collector
./install.sh
cd ../../..

# Loki 배포
echo "2. Loki 배포 중..."
cd deployments/monitoring/loki
./install.sh
cd ../../..

# Tempo 배포
echo "3. Tempo 배포 중..."
cd deployments/monitoring/tempo
./install.sh
cd ../../..

# Prometheus 및 Grafana 배포
echo "4. Prometheus 및 Grafana 배포 중..."
cd deployments/monitoring/prometheus
./install.sh
cd ../../..

echo "모니터링 스택 배포 완료"

