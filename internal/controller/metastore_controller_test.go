/*
Copyright 2023 zncdatadev.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zncdatadev/operator-go/pkg/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
)

var _ = Describe("HiveMetastore controller", func() {
	const (
		clusterName = "test-hive"
		namespace   = "default"
	)

	resourceName := clusterName + "-metastore-default"
	crKey := types.NamespacedName{Name: clusterName, Namespace: namespace}

	newReconciler := func() *reconciler.GenericReconciler[*hivev1alpha1.HiveMetastore] {
		handler := NewHiveRoleGroupHandler(scheme.Scheme)
		r, err := reconciler.NewGenericReconciler(
			&reconciler.GenericReconcilerConfig[*hivev1alpha1.HiveMetastore]{
				Client:           k8sClient,
				Scheme:           scheme.Scheme,
				Recorder:         record.NewFakeRecorder(1024),
				RoleGroupHandler: handler,
				ServiceAccountNameFunc: func(cr *hivev1alpha1.HiveMetastore) string {
					return hivev1alpha1.DefaultProductName + "-" + cr.GetName()
				},
				Prototype: &hivev1alpha1.HiveMetastore{},
			})
		Expect(err).NotTo(HaveOccurred())
		return r
	}

	It("should reconcile a minimal derby cluster into the expected resources", func() {
		cr := &hivev1alpha1.HiveMetastore{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: namespace},
			Spec: hivev1alpha1.HiveMetastoreSpec{
				ClusterConfig: &hivev1alpha1.ClusterConfigSpec{
					Database: &hivev1alpha1.DatabaseSpec{
						ConnString:        "jdbc:derby:;databaseName=/tmp/hive;create=true",
						DatabaseType:      "derby",
						CredentialsSecret: "hive-credentials",
					},
				},
				Metastore: &hivev1alpha1.RoleSpec{
					RoleGroups: map[string]*hivev1alpha1.RoleGroupSpec{
						"default": {Replicas: 1},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, cr)).To(Succeed())

		r := newReconciler()
		req := ctrl.Request{NamespacedName: crKey}
		// The reconciler may requeue between phases (ServiceAccount, resources, status); a few
		// passes reach steady state.
		for range 5 {
			_, err := r.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		}

		By("creating the role group StatefulSet with the metastore container")
		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, sts)).To(Succeed())
		Expect(sts.Spec.Template.Spec.Containers).NotTo(BeEmpty())
		main := sts.Spec.Template.Spec.Containers[0]
		Expect(main.Name).To(Equal("metastore"))
		Expect(main.Args).To(HaveLen(1))
		Expect(main.Args[0]).To(ContainSubstring("exec bin/start-metastore"))
		Expect(main.EnvFrom).To(ContainElement(corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "hive-credentials"},
			},
		}))

		By("creating the role group ConfigMap with hive-site.xml and the log4j2 config")
		cm := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, cm)).To(Succeed())
		Expect(cm.Data).To(HaveKey("hive-site.xml"))
		Expect(cm.Data["hive-site.xml"]).To(ContainSubstring("hive.metastore.warehouse.dir"))
		Expect(cm.Data).To(HaveKey(LogConfigFileName))

		By("creating the client and headless Services")
		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace}, svc)).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName + "-headless", Namespace: namespace}, svc)).To(Succeed())

		By("creating the prometheus-annotated metrics Service")
		metricsSvc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName + "-metrics", Namespace: namespace}, metricsSvc)).To(Succeed())
		Expect(metricsSvc.Annotations).To(HaveKeyWithValue("prometheus.io/scrape", "true"))
		Expect(metricsSvc.Annotations).To(HaveKeyWithValue("prometheus.io/port", "9084"))
		Expect(metricsSvc.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone))

		By("tracking the role group in the CR status")
		updated := &hivev1alpha1.HiveMetastore{}
		Expect(k8sClient.Get(ctx, crKey, updated)).To(Succeed())
		Expect(updated.Status.RoleGroups).To(HaveKeyWithValue("metastore", ContainElement("default")))
	})
})
