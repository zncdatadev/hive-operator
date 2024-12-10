helm upgrade --install hive \
  --version=12.5.6 \
  --namespace $NAMESPACE \
  -f ../setup/helm-bitnami-postgresql-values.yaml \
  --repo https://charts.bitnami.com/bitnami postgresql \
