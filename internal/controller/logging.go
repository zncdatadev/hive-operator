package controller

import (
	"context"
	"time"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	DefaultLog4jProperties = `# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

status = INFO
name = HiveLog4j2
packages = org.apache.hadoop.hive.ql.log

# list of properties
property.hive.log.level = INFO
property.hive.root.logger = DRFA
property.hive.log.dir = ${sys:java.io.tmpdir}/${sys:user.name}
property.hive.log.file = hive.log
property.hive.perflogger.log.level = INFO

# list of all appenders
appenders = console, DRFA

# console appender
appender.console.type = Console
appender.console.name = console
appender.console.target = SYSTEM_ERR
appender.console.layout.type = PatternLayout
appender.console.layout.pattern = %d{ISO8601} %5p [%t] %c{2}: %m%n

# daily rolling file appender
appender.DRFA.type = RollingRandomAccessFile
appender.DRFA.name = DRFA
appender.DRFA.fileName = ${sys:hive.log.dir}/${sys:hive.log.file}
# Use %pid in the filePattern to append <process-id>@<host-name> to the filename if you want separate log files for different CLI session
appender.DRFA.filePattern = ${sys:hive.log.dir}/${sys:hive.log.file}.%d{yyyy-MM-dd}
appender.DRFA.layout.type = PatternLayout
appender.DRFA.layout.pattern = %d{ISO8601} %5p [%t] %c{2}: %m%n
appender.DRFA.policies.type = Policies
appender.DRFA.policies.time.type = TimeBasedTriggeringPolicy
appender.DRFA.policies.time.interval = 1
appender.DRFA.policies.time.modulate = true
appender.DRFA.strategy.type = DefaultRolloverStrategy
appender.DRFA.strategy.max = 30

# list of all loggers
loggers = NIOServerCnxn, ClientCnxnSocketNIO, DataNucleus, Datastore, JPOX, PerfLogger, AmazonAws, ApacheHttp

logger.NIOServerCnxn.name = org.apache.zookeeper.server.NIOServerCnxn
logger.NIOServerCnxn.level = WARN

logger.ClientCnxnSocketNIO.name = org.apache.zookeeper.ClientCnxnSocketNIO
logger.ClientCnxnSocketNIO.level = WARN

logger.DataNucleus.name = DataNucleus
logger.DataNucleus.level = ERROR

logger.Datastore.name = Datastore
logger.Datastore.level = ERROR

logger.JPOX.name = JPOX
logger.JPOX.level = ERROR

logger.AmazonAws.name=com.amazonaws
logger.AmazonAws.level = INFO

logger.ApacheHttp.name=org.apache.http
logger.ApacheHttp.level = INFO

logger.PerfLogger.name = org.apache.hadoop.hive.ql.log.PerfLogger
logger.PerfLogger.level = ${sys:hive.perflogger.log.level}

# root logger
rootLogger.level = ${sys:hive.log.level}
rootLogger.appenderRefs = root
rootLogger.appenderRef.root.ref = ${sys:hive.root.logger}
`
	HiveMetastoreLog4jName = "hive-log4j2.properties"
)

type MetastoreLoggingRecociler struct {
	BaseRoleGroupResourceReconciler
}

func NewLog4jConfigMapRecociler(
	client client.Client,
	scheme *runtime.Scheme,
	cr *hivev1alpha1.HiveMetastore,
	roleName string,
	roleGroupName string,
	roleGroup *hivev1alpha1.RoleGroupSpec,
) *MetastoreLoggingRecociler {
	return &MetastoreLoggingRecociler{
		BaseRoleGroupResourceReconciler{
			client:        client,
			scheme:        scheme,
			cr:            cr,
			roleName:      roleName,
			roleGroupName: roleGroupName,
			roleGroup:     roleGroup,
		},
	}
}

func MetastoreLog4jConfigMapName(cr *hivev1alpha1.HiveMetastore, roleGroupName string) string {
	return cr.GetNameWithSuffix(roleGroupName + "-log4j2")
}

func (r *MetastoreLoggingRecociler) Enable() bool {
	return r.roleGroup.Config != nil &&
		r.roleGroup.Config.Logging != nil &&
		r.roleGroup.Config.Logging.Metastore != nil
}

func (r *MetastoreLoggingRecociler) Name() string {
	return MetastoreLog4jConfigMapName(r.cr, r.roleGroupName)
}

func (r *MetastoreLoggingRecociler) make(data map[string]string) (*corev1.ConfigMap, error) {
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.Name(),
			Namespace: r.cr.Namespace,
			Labels:    r.cr.GetLabels(),
		},
		Data: data,
	}

	if err := controllerutil.SetControllerReference(r.cr, obj, r.scheme); err != nil {
		return nil, err
	}

	return obj, nil
}

func (r *MetastoreLoggingRecociler) Reconcile(ctx context.Context) (ctrl.Result, error) {
	if !r.Enable() {
		log.Info(
			"Logging configuration is not enabled for role group, so skip.", "roleGroup",
			r.roleGroupName,
		)
		return ctrl.Result{}, nil
	}

	log.Info("Reconciling Logging to configmap")

	if res, err := r.apply(ctx); err != nil {
		log.Error(err, "Failed to reconcile Logging to configmap")
		return ctrl.Result{}, err
	} else if res.RequeueAfter > 0 {
		return res, nil
	}

	return ctrl.Result{}, nil

}

func (r *MetastoreLoggingRecociler) apply(ctx context.Context) (ctrl.Result, error) {
	data := make(map[string]string)

	if r.roleGroup.Config.Logging.Metastore != nil {
		data[HiveMetastoreLog4jName] = r.metastoreLog4j(r.roleGroup.Config.Logging.Metastore)
	}

	obj, err := r.make(data)
	if err != nil {
		return ctrl.Result{}, err
	}

	if mutant, err := CreateOrUpdate(ctx, r.client, obj); err != nil {
		return ctrl.Result{}, err
	} else if mutant {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *MetastoreLoggingRecociler) metastoreLog4j(loggingConfig *hivev1alpha1.LoggingConfigSpec) string {
	properties := make(map[string]string)

	if loggingConfig.Loggers != nil {
		for k, level := range loggingConfig.Loggers {
			if level != nil {
				v := *level
				properties["logger."+k+".level"] = v.Level
			}
		}
	}
	if loggingConfig.Console != nil {
		properties["appender.console.filter.threshold.type"] = "ThresholdFilter"
		properties["appender.console.filter.threshold.level"] = loggingConfig.Console.Level
	}

	if loggingConfig.File != nil {
		properties["appender.DRFA.filter.threshold.type"] = "ThresholdFilter"
		properties["appender.DRFA.filter.threshold.level"] = loggingConfig.File.Level
	}

	props := log4jProperties(properties)

	return DefaultLog4jProperties +
		"\n\n" +
		"# hive-operator modify logging\n" +
		props
}

func log4jProperties(properties map[string]string) string {
	data := ""
	for k, v := range properties {
		data += k + "=" + v + "\n"
	}
	return data
}
