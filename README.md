# Kubedoop Operator for Apache Hive

[![Build](https://github.com/zncdatadev/hive-operator/actions/workflows/publish.yml/badge.svg)](https://github.com/zncdatadev/hive-operator/actions/workflows/publish.yml)
[![LICENSE](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/zncdatadev/hive-operator)](https://goreportcard.com/report/github.com/zncdatadev/hive-operator)
[![Artifact HUB](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/hive-operator)](https://artifacthub.io/packages/helm/kubedoop/hive-operator)

This is a Kubernetes operator to manage Apache Hive.

It's part of the kubedoop Data Platform, a modular open source data platform built on Kubernetes that provides Kubernetes native deployment
and management of popular open source data apps like Apache Kafka, Apache Doris, Apache Kyuubi, Trino or Apache Spark, all working
together seamlessly. Based on Kubernetes, it runs everywhere – on prem or in the cloud.

## Quick Start

### Install Requirements Dependencies

> Please make sure helm version is v3.8.0+

```bash
helm install commons-operator oci://quay.io/kubedoopcharts/commons-operator
helm install listener-operator oci://quay.io/kubedoopcharts/listener-operator
helm install secret-operator oci://quay.io/kubedoopcharts/secret-operator
```

### Install hive-operator

```bash
helm install hive-operator oci://quay.io/kubedoopcharts/hive-operator
```

### Deploy hive cluster

```bash
kubectl apply -f config/samples
```

## Kubedoop Data Platform Operators

These are the operators that are currently part of the Kubedoop Data Platform:

- [Kubedoop Operator for Apache Airflow](https://github.com/zncdatadev/airflow-operator)
- [Kubedoop Operator for Apache DolphinScheduler](https://github.com/zncdatadev/dolphinscheduler-operator)
- [Kubedoop Operator for Apache Doris](https://github.com/zncdatadev/doris-operator)
- [Kubedoop Operator for Apache Hadoop HDFS](https://github.com/zncdatadev/hdfs-operator)
- [Kubedoop Operator for Apache HBase](https://github.com/zncdatadev/hbase-operator)
- [Kubedoop Operator for Apache Hive](https://github.com/zncdatadev/hive-operator)
- [Kubedoop Operator for Apache Kafka](https://github.com/zncdatadev/kafka-operator)
- [Kubedoop Operator for Apache Kyuubi](https://github.com/zncdatadev/kyuubi-operator)
- [Kubedoop Operator for Apache Nifi](https://github.com/zncdatadev/nifi-operator)
- [Kubedoop Operator for Apache Spark](https://github.com/zncdatadev/spark-k8s-operator)
- [Kubedoop Operator for Apache Superset](https://github.com/zncdatadev/superset-operator)
- [Kubedoop Operator for Trino](https://github.com/zncdatadev/trino-operator)
- [Kubedoop Operator for Apache Zookeeper](https://github.com/zncdatadev/zookeeper-operator)

And our internal operators: :

- [Commons Operator](https://github.com/zncdatadev/commons-operator)
- [Listener Operator](https://github.com/zncdatadev/listener-operator)
- [Secret Operator](https://github.com/zncdatadev/secret-operator)

## Contributing

If you'd like to contribute to Kubedoop, please refer to our [Contributing Guide](https://kubedoop.dev/docs/developer-manual/collaboration) for more information.
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
