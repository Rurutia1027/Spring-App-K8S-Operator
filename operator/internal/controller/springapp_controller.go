/*
Copyright 2026.
*/

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/Rurutia1027/Spring-App-K8S-Operator/operator/api/v1alpha1"
)

const (
	finalizerName        = "springapp.apps.example.com/finalizer"
	managedByLabel       = "spring-notes-operator"
	configChecksumAnno   = "apps.example.com/config-checksum"
	defaultContainerPort = 8080
)

// SpringAppReconciler reconciles a SpringApp object.
type SpringAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.example.com,resources=springapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.example.com,resources=springapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.example.com,resources=springapps/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *SpringAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	app := &appsv1alpha1.SpringApp{}
	if err := r.Get(ctx, req.NamespacedName, app); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !app.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, app)
	}

	if !controllerutil.ContainsFinalizer(app, finalizerName) {
		controllerutil.AddFinalizer(app, finalizerName)
		if err := r.Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	dbOK, dbErr := r.validateDatabaseSecret(ctx, app)
	if !dbOK {
		return r.updateStatus(ctx, app, appsv1alpha1.PhaseDegraded, dbErr, nil, nil)
	}

	configChecksum, err := r.reconcileConfigMap(ctx, app)
	if err != nil {
		return r.updateStatus(ctx, app, appsv1alpha1.PhaseDegraded, err, nil, nil)
	}

	if err := r.reconcileService(ctx, app); err != nil {
		return r.updateStatus(ctx, app, appsv1alpha1.PhaseDegraded, err, nil, nil)
	}

	deploy, err := r.reconcileDeployment(ctx, app, configChecksum)
	if err != nil {
		return r.updateStatus(ctx, app, appsv1alpha1.PhaseDegraded, err, nil, nil)
	}

	return r.updateStatus(ctx, app, "", nil, deploy, nil)
}

func (r *SpringAppReconciler) reconcileDelete(ctx context.Context, app *appsv1alpha1.SpringApp) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("reconciling delete", "name", app.Name)

	app.Status.Phase = appsv1alpha1.PhaseDeleting
	_ = r.Status().Update(ctx, app)

	if controllerutil.ContainsFinalizer(app, finalizerName) {
		controllerutil.RemoveFinalizer(app, finalizerName)
		if err := r.Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *SpringAppReconciler) validateDatabaseSecret(ctx context.Context, app *appsv1alpha1.SpringApp) (bool, error) {
	db := app.Spec.Database
	if db.Host == "" || db.Name == "" || db.CredentialsSecretRef.Name == "" {
		return true, nil
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      db.CredentialsSecretRef.Name,
		Namespace: app.Namespace,
	}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return false, fmt.Errorf("database secret %q not found", db.CredentialsSecretRef.Name)
		}
		return false, err
	}

	userKey := app.Spec.GetUsernameKey()
	passKey := app.Spec.GetPasswordKey()
	if len(secret.Data[userKey]) == 0 || len(secret.Data[passKey]) == 0 {
		return false, fmt.Errorf("database secret %q missing keys %q/%q", secret.Name, userKey, passKey)
	}
	return true, nil
}

func (r *SpringAppReconciler) reconcileConfigMap(ctx context.Context, app *appsv1alpha1.SpringApp) (string, error) {
	name := app.Spec.Runtime.ConfigMapName
	if name == "" {
		name = app.Name + "-config"
	}

	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: app.Namespace,
			Labels:    r.labels(app),
		},
		Data: map[string]string{
			"application-k8s.properties": r.configProperties(app),
		},
	}

	if err := controllerutil.SetControllerReference(app, desired, r.Scheme); err != nil {
		return "", err
	}

	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return "", err
		}
		return checksumMap(desired.Data), nil
	}
	if err != nil {
		return "", err
	}

	existing.Labels = desired.Labels
	existing.Data = desired.Data
	if err := r.Update(ctx, existing); err != nil {
		return "", err
	}
	return checksumMap(existing.Data), nil
}

func (r *SpringAppReconciler) reconcileService(ctx context.Context, app *appsv1alpha1.SpringApp) error {
	port := app.Spec.GetServicePort()
	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels:    r.labels(app),
		},
		Spec: corev1.ServiceSpec{
			Type:     app.Spec.GetServiceType(),
			Selector: r.selectorLabels(app),
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       port,
				TargetPort: intstr.FromInt32(defaultContainerPort),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}

	if err := controllerutil.SetControllerReference(app, desired, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	existing.Spec.Type = desired.Spec.Type
	existing.Spec.Ports = desired.Spec.Ports
	existing.Spec.Selector = desired.Spec.Selector
	existing.Labels = desired.Labels
	return r.Update(ctx, existing)
}

func (r *SpringAppReconciler) reconcileDeployment(ctx context.Context, app *appsv1alpha1.SpringApp, configChecksum string) (*appsv1.Deployment, error) {
	replicas := app.Spec.GetReplicas()
	configMapName := app.Spec.Runtime.ConfigMapName
	if configMapName == "" {
		configMapName = app.Name + "-config"
	}

	podLabels := r.selectorLabels(app)
	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: podLabels,
			Annotations: map[string]string{
				configChecksumAnno: configChecksum,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "app",
				Image: app.Spec.Image,
				Ports: []corev1.ContainerPort{{
					Name:          "http",
					ContainerPort: defaultContainerPort,
				}},
				Env:            r.buildEnv(app),
				Resources:      app.Spec.Runtime.Resources,
				LivenessProbe:  httpProbe("/actuator/health/liveness"),
				ReadinessProbe: httpProbe("/actuator/health/readiness"),
			}},
			Volumes: []corev1.Volume{{
				Name: "config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
					},
				},
			}},
		},
	}
	podTemplate.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{
		Name:      "config",
		MountPath: "/config",
	}}
	if app.Spec.Runtime.JvmOptions != "" {
		podTemplate.Spec.Containers[0].Env = append(podTemplate.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "JAVA_TOOL_OPTIONS",
			Value: app.Spec.Runtime.JvmOptions,
		})
	}

	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels:    r.labels(app),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: podLabels},
			Template: podTemplate,
		},
	}

	if err := controllerutil.SetControllerReference(app, desired, r.Scheme); err != nil {
		return nil, err
	}

	existing := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return nil, err
		}
		return desired, nil
	}
	if err != nil {
		return nil, err
	}

	if app.Spec.Release.Paused {
		return existing, nil
	}

	existing.Labels = desired.Labels
	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Selector = desired.Spec.Selector
	existing.Spec.Template = desired.Spec.Template
	if err := r.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (r *SpringAppReconciler) updateStatus(
	ctx context.Context,
	app *appsv1alpha1.SpringApp,
	forcedPhase string,
	reconcileErr error,
	deploy *appsv1.Deployment,
	_ *corev1.Service,
) (ctrl.Result, error) {
	latest := &appsv1alpha1.SpringApp{}
	if err := r.Get(ctx, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}, latest); err != nil {
		return ctrl.Result{}, err
	}

	latest.Status.ObservedGeneration = latest.Generation
	latest.Status.CurrentImage = latest.Spec.Image

	if reconcileErr != nil {
		latest.Status.Phase = appsv1alpha1.PhaseDegraded
		latest.Status.LastError = reconcileErr.Error()
		latest.Status.DatabaseConnectivity = "Error"
		setCondition(&latest.Status, appsv1alpha1.ConditionReady, metav1.ConditionFalse, "ReconcileError", reconcileErr.Error())
		setCondition(&latest.Status, appsv1alpha1.ConditionDatabaseReady, metav1.ConditionFalse, "DatabaseConfigError", reconcileErr.Error())
		return ctrl.Result{}, r.Status().Update(ctx, latest)
	}

	if deploy == nil {
		deploy = &appsv1.Deployment{}
		_ = r.Get(ctx, types.NamespacedName{Name: latest.Name, Namespace: latest.Namespace}, deploy)
	}

	latest.Status.AvailableReplicas = deploy.Status.AvailableReplicas
	latest.Status.ReadyReplicas = deploy.Status.ReadyReplicas
	latest.Status.LastError = ""

	dbOK, _ := r.validateDatabaseSecret(ctx, latest)
	if latest.Spec.Database.Host != "" {
		if dbOK {
			latest.Status.DatabaseConnectivity = "OK"
			setCondition(&latest.Status, appsv1alpha1.ConditionDatabaseReady, metav1.ConditionTrue, "SecretValid", "database secret is present")
		} else {
			latest.Status.DatabaseConnectivity = "Error"
			setCondition(&latest.Status, appsv1alpha1.ConditionDatabaseReady, metav1.ConditionFalse,
				"SecretInvalid", "database secret validation failed")
		}
	} else {
		latest.Status.DatabaseConnectivity = "Unknown"
	}

	ready := deploy.Status.ReadyReplicas >= latest.Spec.GetReplicas() && deploy.Status.ReadyReplicas > 0
	if forcedPhase != "" {
		latest.Status.Phase = forcedPhase
	} else if latest.Spec.Release.Paused {
		latest.Status.Phase = appsv1alpha1.PhaseProgressing
		setCondition(&latest.Status, appsv1alpha1.ConditionProgressing, metav1.ConditionTrue, "Paused", "release is paused")
	} else if ready {
		latest.Status.Phase = appsv1alpha1.PhaseReady
		setCondition(&latest.Status, appsv1alpha1.ConditionReady, metav1.ConditionTrue, "MinimumReplicasAvailable", "deployment is ready")
		setCondition(&latest.Status, appsv1alpha1.ConditionProgressing, metav1.ConditionFalse, "Completed", "rollout completed")
	} else {
		latest.Status.Phase = appsv1alpha1.PhaseProgressing
		setCondition(&latest.Status, appsv1alpha1.ConditionReady, metav1.ConditionFalse, "Progressing", "waiting for deployment")
		setCondition(&latest.Status, appsv1alpha1.ConditionProgressing, metav1.ConditionTrue, "ReplicaSetUpdated", "rollout in progress")
	}

	if err := r.Status().Update(ctx, latest); err != nil {
		return ctrl.Result{}, err
	}
	if !ready && !latest.Spec.Release.Paused {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

func (r *SpringAppReconciler) buildEnv(app *appsv1alpha1.SpringApp) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: "SPRING_CONFIG_ADDITIONAL_LOCATION", Value: "file:/config/"},
		{Name: "SERVER_PORT", Value: "8080"},
	}
	for k, v := range app.Spec.Runtime.Env {
		env = append(env, corev1.EnvVar{Name: k, Value: v})
	}

	db := app.Spec.Database
	if db.Host != "" && db.Name != "" {
		url := fmt.Sprintf("jdbc:postgresql://%s:%d/%s", db.Host, app.Spec.GetDBPort(), db.Name)
		env = append(env, corev1.EnvVar{Name: "SPRING_DATASOURCE_URL", Value: url})
	}
	if db.CredentialsSecretRef.Name != "" {
		env = append(env,
			corev1.EnvVar{
				Name: "SPRING_DATASOURCE_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: db.CredentialsSecretRef.Name},
						Key:                  app.Spec.GetUsernameKey(),
					},
				},
			},
			corev1.EnvVar{
				Name: "SPRING_DATASOURCE_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: db.CredentialsSecretRef.Name},
						Key:                  app.Spec.GetPasswordKey(),
					},
				},
			},
		)
	}
	return env
}

func (r *SpringAppReconciler) configProperties(app *appsv1alpha1.SpringApp) string {
	profile := "k8s"
	if v, ok := app.Spec.Runtime.Env["SPRING_PROFILES_ACTIVE"]; ok && v != "" {
		profile = v
	}
	lines := []string{
		"spring.application.name=" + app.Name,
		"spring.profiles.active=" + profile,
		"spring.jpa.hibernate.ddl-auto=update",
	}
	if level, ok := app.Spec.Runtime.Env["APP_LOG_LEVEL"]; ok {
		lines = append(lines, "logging.level.root="+level)
	}
	return strings.Join(lines, "\n") + "\n"
}

func (r *SpringAppReconciler) labels(app *appsv1alpha1.SpringApp) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       app.Name,
		"app.kubernetes.io/managed-by": managedByLabel,
		"apps.example.com/springapp":   app.Name,
	}
}

func (r *SpringAppReconciler) selectorLabels(app *appsv1alpha1.SpringApp) map[string]string {
	return map[string]string{
		"apps.example.com/springapp": app.Name,
	}
}

func httpProbe(path string) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt32(defaultContainerPort),
			},
		},
		InitialDelaySeconds: 20,
		PeriodSeconds:       10,
	}
}

func checksumMap(data map[string]string) string {
	h := sha256.New()
	for k, v := range data {
		h.Write([]byte(k))
		h.Write([]byte(v))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func setCondition(status *appsv1alpha1.SpringAppStatus, condType string, condStatus metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	for i := range status.Conditions {
		if status.Conditions[i].Type == condType {
			status.Conditions[i].Status = condStatus
			status.Conditions[i].Reason = reason
			status.Conditions[i].Message = message
			status.Conditions[i].LastTransitionTime = now
			return
		}
	}
	status.Conditions = append(status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             condStatus,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	})
}

func (r *SpringAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.SpringApp{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Named("springapp").
		Complete(r)
}
