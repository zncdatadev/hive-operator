# Operator Chart for Apache Hive on Kubedoop

This chart bootstraps an Apache Hive Operator deployment on a Kubernetes cluster using the Helm package manager. It's part of the Kubedoop ecosystem.

## Pre-Requisites

### Custom resource definitions

Some users would prefer to install the CRDs _outside_ of the chart. You can disable the CRD installation of this chart by using `--set crds.install=false` when installing the chart.

Helm cannot upgrade custom resource definitions in the `<chart>/crds` folder [by design](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations).
Starting with 3.4.0 (chart version 0.19.0), the CRDs have been moved to `<chart>/templates` to address this design decision.

If you are using Argo Workflows chart version prior to 3.4.0 (chart version 0.19.0) or have elected to manage the Argo Workflows CRDs outside of the chart,
please use `kubectl` to upgrade CRDs manually from [templates/crds](templates/crds/) folder or via the manifests from the upstream project repo:

## Installing the Chart

To install the chart with the release name `hive-operator`:

```bash
helm install hive-operator oci://quay.io/kubedoopcharts/hive-operator
```

## Usage

The operator example usage can be found in the [examples](https://github.com/zncdatadev/hive-operator/tree/main/examples) directory.

## More information

- [Kubedoop operator for Apache Hive](https://github.com/zncdatadev/hive-operator)
