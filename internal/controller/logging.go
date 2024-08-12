package controller

import (
	"context"
	"strings"
	"time"

	"golang.org/x/exp/maps"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	ConsoleConversionPattern = "%d{ISO8601} %5p [%t] %c{2}: %m%n"
	Log4j2PropertiesTemplate = `appenders = FILE, CONSOLE

appender.CONSOLE.type = Console
appender.CONSOLE.name = CONSOLE
appender.CONSOLE.target = SYSTEM_ERR
appender.CONSOLE.layout.type = PatternLayout
appender.CONSOLE.layout.pattern = {{.ConsoleConversionPattern}}
appender.CONSOLE.filter.threshold.type = ThresholdFilter
appender.CONSOLE.filter.threshold.level = {{.ConsoleLevel}}

appender.FILE.type = RollingFile
appender.FILE.name = FILE
appender.FILE.fileName = {{.LogDir}}/{{.LogFileName}}
appender.FILE.filePattern = {{.LogDir}}/{{.LogFileName}}.%i
appender.FILE.layout.type = XMLLayout
appender.FILE.policies.type = Policies
appender.FILE.policies.size.type = SizeBasedTriggeringPolicy
appender.FILE.policies.size.size = {{.MaxLogFileSize}}
appender.FILE.strategy.type = DefaultRolloverStrategy
appender.FILE.strategy.max = 1
appender.FILE.filter.threshold.type = ThresholdFilter
appender.FILE.filter.threshold.level = {{.FileLevel}}
{{ if .LoggerNames }}
loggers = {{.LoggerNames}}
{{ end -}}
{{range $name, $levelSpec := .Loggers}}
logger.{{ $name }}.name = {{ $name }}
logger.{{ $name }}.level = {{ $levelSpec.Level -}}
{{ end }}

rootLogger.level = INFO
rootLogger.appenderRefs = CONSOLE, FILE
rootLogger.appenderRef.CONSOLE.ref = CONSOLE
rootLogger.appenderRef.FILE.ref = FILE
`
	HiveMetastoreLog4jName = "metastore-log4j2.properties"
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

	data[HiveMetastoreLog4jName] = r.metastoreLog4j(r.roleGroup.Config.Logging)

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

func (r *MetastoreLoggingRecociler) metastoreLog4j(loggingConfig *hivev1alpha1.ContainerLoggingSpec) string {
	data := make(map[string]interface{})
	data["LogDir"] = hivev1alpha1.KubeDataLogDir + "/" + RoleHiveMetaStore
	data["LogFileName"] = "hive.log4j2.xml"
	data["MaxLogFileSize"] = "10MB"

	consoleLevel := "INFO"
	fileLevel := "INFO"
	if loggingConfig != nil && loggingConfig.Metastore != nil {
		consoleLevel = GetLoggerLevel(loggingConfig.Metastore.Console != nil, func() string { return loggingConfig.Metastore.Console.Level }, consoleLevel)
		fileLevel = GetLoggerLevel(loggingConfig.Metastore.File != nil, func() string { return loggingConfig.Metastore.File.Level }, fileLevel)
		loggers := loggingConfig.Metastore.Loggers
		data["LoggerNames"] = strings.Join(maps.Keys(loggers), ",")
		data["Loggers"] = loggers
	}
	data["ConsoleLevel"] = consoleLevel
	data["FileLevel"] = fileLevel

	parser := config.TemplateParser{
		Value:    data,
		Template: Log4j2PropertiesTemplate,
	}
	res, err := parser.Parse()

	if err != nil {
		panic(err)
	}
	return res
}

func GetLoggerLevel(condition bool, trueValFunc func() string, defaultVal string) string {
	if condition {
		trueVal := trueValFunc()
		if strings.TrimSpace(trueVal) != "" {
			return trueVal
		}
	}
	return defaultVal
}
