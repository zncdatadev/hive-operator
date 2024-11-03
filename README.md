# Kubedoop Operator for Apache Hive

[![Build](https://github.com/zncdatadev/hive-operator/actions/workflows/main.yml/badge.svg)](https://github.com/zncdatadev/hive-operator/actions/workflows/main.yml)
[![LICENSE](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/zncdatadev/hive-operator)](https://goreportcard.com/report/github.com/zncdatadev/hive-operator)
[![Artifact HUB](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/hive-operator)](https://artifacthub.io/packages/helm/kubedoop/hive-operator)

This is a kubernetes operator to manage apache hive on kubernetes cluster. It's part of the kubedoop ecosystem.

Kubedoop is a cloud-native big data platform built on Kubernetes, designed to simplify the deployment and management of big data applications on Kubernetes.
It provides a set of pre-configured Operators to easily deploy and manage various big data components such as HDFS, Hive, Spark, Kafka, and more.

## Quick Start

### Add helm repository

> Please make sure helm version is v3.0.0+

```bash
helm repo add kubedoop https://zncdatadev.github.io/kubedoop-helm-charts/
```

### Add required dependencies

```bash
helm install commons-operator kubedoop/commons-operator
helm install listener-operator kubedoop/listener-operator
helm install secret-operator kubedoop/secret-operator
```

### Add hive-operator

```bash
helm install hive-operator kubedoop/hive-operator
```

### Deploy hive cluster

```bash
kubectl apply -f config/samples
```

## Kubedoop Ecosystem

### Operators

Kubedoop operators:

- [Kubedoop Operator for Apache DolphinScheduler](https://github.com/zncdatadev/dolphinscheduler-operator)
- [Kubedoop Operator for Apache Hadoop HDFS](https://github.com/zncdatadev/hdfs-operator)
- [Kubedoop Operator for Apache HBase](https://github.com/zncdatadev/hbase-operator)
- [Kubedoop Operator for Apache Hive](https://github.com/zncdatadev/hive-operator)
- [Kubedoop Operator for Apache Kafka](https://github.com/zncdatadev/kafka-operator)
- [Kubedoop Operator for Apache Spark](https://github.com/zncdatadev/spark-k8s-operator)
- [Kubedoop Operator for Apache Superset](https://github.com/zncdatadev/superset-operator)
- [Kubedoop Operator for Trino](https://github.com/zncdatadev/trino-operator)
- [Kubedoop Operator for Apache Zookeeper](https://github.com/zncdatadev/zookeeper-operator)

Kubedoop built-in operators:

- [Commons Operator](https://github.com/zncdatadev/commons-operator)
- [Listener Operator](https://github.com/zncdatadev/listener-operator)
- [Secret Operator](https://github.com/zncdatadev/secret-operator)

## Contributing

If you'd like to contribute to Kubedoop, please refer to our [Contributing Guide](https://zncdata.dev/docs/developer-manual/collaboration) for more information.
We welcome contributions of all kinds, including but not limited to code, documentation, and use cases.

## License

Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
