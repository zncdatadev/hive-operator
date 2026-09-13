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

	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
)

var _ = Describe("HiveMetastore controller", func() {
	const clusterName = "test-hive"

	resourceName := clusterName + "-metastore-" + testRoleGroup
	crKey := types.NamespacedName{Name: clusterName, Namespace: testNamespace}

	newReconciler := func() *reconciler.GenericReconciler[*hivev1alpha1.HiveMetastore] {
		handler := NewHiveRoleGroupHandler(scheme.Scheme)
		r, err := reconciler.NewGenericReconciler(
			&reconciler.GenericReconcilerConfig[*hivev1alpha1.HiveMetastore]{
				Client:           k8sClient,
				Scheme:           scheme.Scheme,
				Recorder:         record.NewFakeRecorder(1024),
				RoleGroupHandler: handler,
				// Mirror the production wiring in cmd/main.go: the role catalog, the per-role-group
				// contribution and image resolution are all reconciler-level configuration now.
				RoleProvider:      handler,
				RoleGroupResolver: handler,
				ImageResolution: reconciler.ImageResolution{
					ProductName: hivev1alpha1.DefaultProductName,
					Defaults:    ImageDefaults(),
				},
				Prototype: &hivev1alpha1.HiveMetastore{},
			})
		Expect(err).NotTo(HaveOccurred())
		return r
	}

	It("should reconcile a minimal derby cluster into the expected resources", func() {
		cr := &hivev1alpha1.HiveMetastore{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: testNamespace},
			Spec: hivev1alpha1.HiveMetastoreSpec{
				ClusterConfig: &hivev1alpha1.ClusterConfigSpec{
					Database: &hivev1alpha1.DatabaseSpec{
						ConnString:        testDerbyConnString,
						DatabaseType:      databaseTypeDerby,
						CredentialsSecret: testCredentialsSecret,
					},
				},
				Metastore: &hivev1alpha1.RoleSpec{
					RoleGroups: map[string]*hivev1alpha1.RoleGroupSpec{
						testRoleGroup: {Replicas: 1},
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
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: testNamespace}, sts)).To(Succeed())
		Expect(sts.Spec.Template.Spec.Containers).NotTo(BeEmpty())
		main := sts.Spec.Template.Spec.Containers[0]
		Expect(main.Name).To(Equal("metastore"))
		// The start script rides in the entrypoint: the framework has no Args declaration,
		// because cliOverrides is the user's channel for arguments.
		Expect(main.Command).NotTo(BeEmpty())
		Expect(main.Command[len(main.Command)-1]).To(ContainSubstring("exec bin/start-metastore"))
		// The script exports S3 credentials read from mounted files, so xtrace would print the
		// expanded secret values into the container log.
		Expect(main.Command).NotTo(ContainElement("-x"))
		Expect(main.EnvFrom).To(ContainElement(corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: testCredentialsSecret},
			},
		}))

		By("creating the role group ConfigMap with hive-site.xml and the log4j2 config")
		cm := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: testNamespace}, cm)).To(Succeed())
		Expect(cm.Data).To(HaveKey("hive-site.xml"))
		Expect(cm.Data["hive-site.xml"]).To(ContainSubstring("hive.metastore.warehouse.dir"))
		Expect(cm.Data).To(HaveKey(LogConfigFileName))

		By("creating the client and headless Services")
		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: testNamespace}, svc)).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName + "-headless", Namespace: testNamespace}, svc)).To(Succeed())

		By("creating the prometheus-annotated metrics Service")
		metricsSvc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName + "-metrics", Namespace: testNamespace}, metricsSvc)).To(Succeed())
		Expect(metricsSvc.Annotations).To(HaveKeyWithValue("prometheus.io/scrape", "true"))
		Expect(metricsSvc.Annotations).To(HaveKeyWithValue("prometheus.io/port", "9084"))
		Expect(metricsSvc.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone))

		By("tracking the role group in the CR status")
		updated := &hivev1alpha1.HiveMetastore{}
		Expect(k8sClient.Get(ctx, crKey, updated)).To(Succeed())
		Expect(updated.Status.RoleGroups).To(HaveKeyWithValue("metastore", ContainElement(testRoleGroup)))

		By("publishing the recommended name and version labels")
		Expect(sts.Spec.Template.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", hivev1alpha1.DefaultProductName))
		Expect(sts.Spec.Template.Labels).To(HaveKey("app.kubernetes.io/version"))

		By("resolving the image from spec.image and the handler's defaults")
		Expect(main.Image).To(HavePrefix(hivev1alpha1.DefaultRepository + "/hive:"))
		Expect(main.Image).To(ContainSubstring("-kubedoop"))
	})

	// The metastore container used to be patched AFTER the framework merged podOverrides, so a
	// product default silently outranked the user. Its shape is a RoleDeclaration now, which the
	// framework applies before the merge.
	It("lets podOverrides win over the product's container defaults", func() {
		podOverrides := `{"spec":{"containers":[{"name":"metastore","livenessProbe":{"tcpSocket":{"port":"metastore"},"initialDelaySeconds":123}}]}}`
		cr := &hivev1alpha1.HiveMetastore{
			ObjectMeta: metav1.ObjectMeta{Name: "override-precedence", Namespace: testNamespace},
			Spec: hivev1alpha1.HiveMetastoreSpec{
				ClusterConfig: &hivev1alpha1.ClusterConfigSpec{
					Database: &hivev1alpha1.DatabaseSpec{
						ConnString:        testDerbyConnString,
						DatabaseType:      databaseTypeDerby,
						CredentialsSecret: testCredentialsSecret,
					},
				},
				Metastore: &hivev1alpha1.RoleSpec{
					RoleGroups: map[string]*hivev1alpha1.RoleGroupSpec{
						testRoleGroup: {
							Replicas: 1,
							OverridesSpec: &commonsv1alpha1.OverridesSpec{
								PodOverrides: &k8sruntime.RawExtension{Raw: []byte(podOverrides)},
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, cr)).To(Succeed())
		defer func() { Expect(k8sClient.Delete(ctx, cr)).To(Succeed()) }()

		r := newReconciler()
		req := ctrl.Request{NamespacedName: types.NamespacedName{Name: cr.Name, Namespace: testNamespace}}
		for range 5 {
			_, err := r.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		}

		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name: cr.Name + "-metastore-" + testRoleGroup, Namespace: testNamespace,
		}, sts)).To(Succeed())

		var main corev1.Container
		for _, c := range sts.Spec.Template.Spec.Containers {
			if c.Name == "metastore" {
				main = c
			}
		}
		Expect(main.LivenessProbe).NotTo(BeNil())
		Expect(main.LivenessProbe.InitialDelaySeconds).To(Equal(int32(123)))
		// The product's own defaults still apply where the override is silent.
		// The start script rides in the entrypoint: the framework has no Args declaration,
		// because cliOverrides is the user's channel for arguments.
		Expect(main.Command).NotTo(BeEmpty())
		Expect(main.Command[len(main.Command)-1]).To(ContainSubstring("exec bin/start-metastore"))
	})
})
