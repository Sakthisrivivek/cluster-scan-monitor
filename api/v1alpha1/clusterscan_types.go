package v1alpha1

import (
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterScanSpec struct {
	Selector    *metav1.LabelSelector       `json:"selector"`
	Schedule    string                       `json:"schedule,omitempty"`
	JobTemplate batchv1.JobTemplateSpec      `json:"jobTemplate"`
}

type ClusterScanConditionStatus string

const (
	ConditionFailed    ClusterScanConditionStatus = "Failed"
	ConditionSucceeded ClusterScanConditionStatus = "Succeeded"
)

type ClusterScanCondition struct {
	ClusterName       string          `json:"clusterName"`
	Status            ClusterScanConditionStatus `json:"status"`
	LastExecutionTime metav1.Time    `json:"lastExecutionTime"`
}

type ClusterScanStatus struct {
	Conditions []ClusterScanCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type ClusterScan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec               ClusterScanSpec `json:"spec"`
	Status             ClusterScanStatus `json:"status,omitempty"`
}

type ClusterScanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterScan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterScan{}, &ClusterScanList{})
}
