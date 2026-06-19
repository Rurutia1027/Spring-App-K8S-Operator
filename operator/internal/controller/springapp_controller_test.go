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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	newReconciler := func() *SpringAppReconciler {
		return &SpringAppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
	}

	reconcileUntil := func(times int) {
		reconciler := newReconciler()
		for range times {
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
		}
	}

	baseSpec := func() appsv1alpha1.SpringAppSpec {
		return appsv1alpha1.SpringAppSpec{
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
		}
	}

	createDBSecret := func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: dbSecretName, Namespace: namespace},
			StringData: map[string]string{"username": "notes", "password": "notes"},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
	}

	createSpringApp := func(spec appsv1alpha1.SpringAppSpec) {
		app := &appsv1alpha1.SpringApp{
			ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: namespace},
			Spec:       spec,
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
	}

	cleanupSpringApp := func() {
		app := &appsv1alpha1.SpringApp{}
		if err := k8sClient.Get(ctx, typeNamespacedName, app); err == nil {
			app.Finalizers = nil
			_ = k8sClient.Update(ctx, app)
			_ = k8sClient.Delete(ctx, app)
		}
	}

	cleanupDBSecret := func() {
		secret := &corev1.Secret{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: dbSecretName, Namespace: namespace}, secret); err == nil {
			_ = k8sClient.Delete(ctx, secret)
		}
	}

	AfterEach(func() {
		cleanupSpringApp()
		cleanupDBSecret()
	})

	Context("happy path", func() {
		BeforeEach(func() {
			createDBSecret()
			createSpringApp(baseSpec())
		})

		It("creates deployment and service with datasource env and finalizer", func() {
			reconcileUntil(3)

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
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(8080)))

			app := &appsv1alpha1.SpringApp{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, app)).To(Succeed())
			Expect(app.Finalizers).To(ContainElement(finalizerName))
		})

		It("sets Phase Ready when deployment replicas are ready", func() {
			reconcileUntil(3)

			deploy := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, deploy)).To(Succeed())
			deploy.Status.Replicas = 1
			deploy.Status.ReadyReplicas = 1
			deploy.Status.AvailableReplicas = 1
			Expect(k8sClient.Status().Update(ctx, deploy)).To(Succeed())

			reconcileUntil(1)

			app := &appsv1alpha1.SpringApp{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, app)).To(Succeed())
			Expect(app.Status.Phase).To(Equal(appsv1alpha1.PhaseReady))
			Expect(app.Status.ReadyReplicas).To(Equal(int32(1)))
		})
	})

	Context("database secret validation", func() {
		BeforeEach(func() {
			createSpringApp(baseSpec())
		})

		It("sets Phase Degraded when database secret is missing", func() {
			reconcileUntil(3)

			app := &appsv1alpha1.SpringApp{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, app)).To(Succeed())
			Expect(app.Status.Phase).To(Equal(appsv1alpha1.PhaseDegraded))
			Expect(app.Status.LastError).To(ContainSubstring("not found"))
		})
	})

	Context("configmap", func() {
		BeforeEach(func() {
			createDBSecret()
			spec := baseSpec()
			spec.Runtime.ConfigMapName = resourceName + "-config"
			createSpringApp(spec)
		})

		It("creates configmap when runtime.configMapName is set", func() {
			reconcileUntil(3)

			cm := &corev1.ConfigMap{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: resourceName + "-config", Namespace: namespace,
			}, cm)).To(Succeed())
			Expect(cm.Data).To(HaveKey("application-prod.properties"))
			Expect(cm.Data["application-prod.properties"]).To(ContainSubstring("spring.application.name=test-notes"))

			deploy := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, deploy)).To(Succeed())
			Expect(deploy.Spec.Template.Spec.Volumes).NotTo(BeEmpty())
		})
	})

	Context("delete", func() {
		BeforeEach(func() {
			createDBSecret()
			createSpringApp(baseSpec())
			reconcileUntil(3)
		})

		It("removes finalizer on delete so the SpringApp can be garbage collected", func() {
			app := &appsv1alpha1.SpringApp{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, app)).To(Succeed())
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())

			_, err := newReconciler().Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, typeNamespacedName, app)
			Expect(apierrors.IsNotFound(err)).To(BeTrue())

			deploy := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, deploy)).To(Succeed())
			Expect(deploy.OwnerReferences).NotTo(BeEmpty())
			Expect(deploy.OwnerReferences[0].Name).To(Equal(resourceName))
		})
	})
})
