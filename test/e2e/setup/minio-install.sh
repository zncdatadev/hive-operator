helm upgrade --install minio \
  --namespace $NAMESPACE \
  --version 12.6.4 \
  -f ../setup/helm-bitnami-minio-values.yaml \
  --repo https://charts.bitnami.com/bitnami minio
