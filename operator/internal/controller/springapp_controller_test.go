/*
Copyright 2026.
*/

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/Rurutia1027/Spring-App-K8S-Operator/operator/api/v1alpha1"
)

var _ = Describe("SpringApp Controller", func() {
	const (
		resourceName = "test-notes"
		namespace    = "default"
		dbSecretName = "notes-db-secret"
	)

	ctx := context.Background()
	typeNamespacedName := types.NamespacedName{Name: resourceName, Namespace: namespace}

	replicas := int32(1)

	BeforeEach(func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: dbSecretName, Namespace: namespace},
			StringData: map[string]string{"username": "notes", "password": "notes"},
		}
		_ = k8sClient.Create(ctx, secret)

		app := &appsv1alpha1.SpringApp{
			ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: namespace},
			Spec: appsv1alpha1.SpringAppSpec{
				Image:    "notes-service:dev",
				Replicas: &replicas,
				Database: appsv1alpha1.DatabaseSpec{
					Provider: "postgres",
					Host:     "postgres.demo.svc.cluster.local",
					Port:     5432,
					Name:     "notesdb",
					CredentialsSecretRef: appsv1alpha1.SecretKeyRef{
						Name: dbSecretName,
					},
				},
				Runtime: appsv1alpha1.RuntimeSpec{
					Env: map[string]string{"APP_LOG_LEVEL": "INFO"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
	})

	AfterEach(func() {
		app := &appsv1alpha1.SpringApp{}
		if err := k8sClient.Get(ctx, typeNamespacedName, app); err == nil {
			app.Finalizers = nil
			_ = k8sClient.Update(ctx, app)
			_ = k8sClient.Delete(ctx, app)
		}
		secret := &corev1.Secret{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: dbSecretName, Namespace: namespace}, secret); err == nil {
			_ = k8sClient.Delete(ctx, secret)
		}
	})

	It("creates deployment and service with datasource env", func() {
		reconciler := &SpringAppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		for range 3 {
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
		}

		deploy := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, typeNamespacedName, deploy)).To(Succeed())
		Expect(deploy.Spec.Template.Spec.Volumes).To(BeEmpty())

		envNames := map[string]string{}
		for _, e := range deploy.Spec.Template.Spec.Containers[0].Env {
			envNames[e.Name] = e.Value
		}
		Expect(envNames).To(HaveKeyWithValue("SPRING_DATASOURCE_URL",
			"jdbc:postgresql://postgres.demo.svc.cluster.local:5432/notesdb"))

		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, typeNamespacedName, svc)).To(Succeed())

		app := &appsv1alpha1.SpringApp{}
		Expect(k8sClient.Get(ctx, typeNamespacedName, app)).To(Succeed())
		Expect(app.Finalizers).To(ContainElement(finalizerName))
	})
})
