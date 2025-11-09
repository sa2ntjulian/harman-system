#!/bin/bash
set -e

echo "Prometheus 및 Grafana 배포 중..."

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -

helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --values values.yaml \
  --wait \
  --timeout 10m

echo "Grafana 접근: minikube service -n monitoring prometheus-grafana --url"
echo "Prometheus 접근: kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090"

