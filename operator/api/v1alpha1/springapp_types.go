/*
Copyright 2026.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PhasePending     = "Pending"
	PhaseProgressing = "Progressing"
	PhaseReady       = "Ready"
	PhaseDegraded    = "Degraded"
	PhaseDeleting    = "Deleting"

	ConditionReady         = "Ready"
	ConditionDatabaseReady = "DatabaseReady"
	ConditionProgressing   = "Progressing"
)

// SpringAppSpec defines the desired state of SpringApp
type SpringAppSpec struct {
	// Image is the Spring Boot container image.
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Replicas is the desired number of pods
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	Service ServiceSpec `json:"service,omitempty"`

	Database DatabaseSpec `json:"database,omitempty"`

	Runtime RuntimeSpec `json:"runtime,omitempty"`

	Release ReleaseSpec `json:"release,omitempty"`
}

type ServiceSpec struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=8080
	Port int32 `json:"port,omitempty"`

	// +kubebuilder:validation:Enum=ClusterIP;NodePort
	// +kubebuilder:default=ClusterIP
	Type corev1.ServiceType `json:"type,omitempty"`
}

type DatabaseSpec struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=5432
	Port int32  `json:"port,omitempty"`
	Name string `json:"name,omitempty"`

	CredentialsSecretRef SecretKeyRef `json:"credentialsSecretRef,omitempty"`
}

type SecretKeyRef struct {
	Name        string `json:"name,omitempty"`
	UsernameKey string `json:"usernameKey,omitempty"`
	PasswordKey string `json:"passwordKey,omitempty"`
}

type RuntimeSpec struct {
	Env           map[string]string           `json:"env,omitempty"`
	Resources     corev1.ResourceRequirements `json:"resources,omitempty"`
	JvmOptions    string                      `json:"jvmOptions,omitempty"`
	ConfigMapName string                      `json:"configMapName,omitempty"`
}

type ReleaseSpec struct {
	Paused bool `json:"paused,omitempty"`
}

// SpringAppStatus defines the observed state of SpringApp.
type SpringAppStatus struct {
	ObservedGeneration   int64              `json:"observedGeneration,omitempty"`
	Phase                string             `json:"phase,omitempty"`
	AvailableReplicas    int32              `json:"availableReplicas,omitempty"`
	ReadyReplicas        int32              `json:"readyReplicas,omitempty"`
	CurrentImage         string             `json:"currentImage,omitempty"`
	DatabaseConnectivity string             `json:"databaseConnectivity,omitempty"`
	LastError            string             `json:"lastError,omitempty"`
	Conditions           []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image`
// +kubebuilder:resource:shortName=sapp

// SpringApp is the Schema for the springapps API
type SpringApp struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of SpringApp
	// +required
	Spec SpringAppSpec `json:"spec"`

	// status defines the observed state of SpringApp
	// +optional
	Status SpringAppStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SpringAppList contains a list of SpringApp
type SpringAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []SpringApp `json:"items"`
}

func (s *SpringAppSpec) GetReplicas() int32 {
	if s.Replicas == nil {
		return 1
	}
	return *s.Replicas
}

func (s *SpringAppSpec) GetServicePort() int32 {
	if s.Service.Port == 0 {
		return 8080
	}
	return s.Service.Port
}

func (s *SpringAppSpec) GetServiceType() corev1.ServiceType {
	if s.Service.Type == "" {
		return corev1.ServiceTypeClusterIP
	}
	return s.Service.Type
}

func (s *SpringAppSpec) GetDBPort() int32 {
	if s.Database.Port == 0 {
		return 5432
	}
	return s.Database.Port
}

func (s *SpringAppSpec) GetUsernameKey() string {
	if s.Database.CredentialsSecretRef.UsernameKey == "" {
		return "username"
	}
	return s.Database.CredentialsSecretRef.UsernameKey
}

func (s *SpringAppSpec) GetPasswordKey() string {
	if s.Database.CredentialsSecretRef.PasswordKey == "" {
		return "password"
	}
	return s.Database.CredentialsSecretRef.PasswordKey
}
