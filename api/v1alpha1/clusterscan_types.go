/*
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
*/

package v1alpha1

import (
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterScanSpec defines the desired state of ClusterScan
type ClusterScanSpec struct {
	// LabelSelector is used to find matching Secrets that store credentials.
	Selector *metav1.LabelSelector `json:"selector"`

	// The schedule in Cron format.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Specifies the job that will be created when executing a Job/CronJob.
	JobTemplate batchv1.JobTemplateSpec `json:"jobTemplate"`
}

type ClusterScanConditionStatus string

const (
	ConditionFailed    ClusterScanConditionStatus = "Failed"
	ConditionSucceeded ClusterScanConditionStatus = "Succeeded"
)

type ClusterScanCondition struct {
	// The name of the cluster being scanned.
	ClusterName string `json:"clusterName"`

	// The outcome of the job (e.g., succeeded, failed).
	Status ClusterScanConditionStatus `json:"status"`

	// The timestamp of the last job execution.
	LastExecutionTime metav1.Time `json:"lastExecutionTime"`
}

// ClusterScanStatus represents the current state of a ClusterScan
type ClusterScanStatus struct {
	// The latest available observations of an object's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=atomic
	Conditions []ClusterScanCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resourse:path=clusterscans,shortName=clscn
//+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//+k8s:deepcopy-gen=true

// ClusterScan is the Schema for the ClusterScan API
type ClusterScan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behaviour of the ClusterScan
	Spec ClusterScanSpec `json:"spec"`

	// Most recently observed status of ClusterScan
	Status ClusterScanStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterScanList contains a list of ClusterScan
type ClusterScanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterScan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterScan{}, &ClusterScanList{})
}
