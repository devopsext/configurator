package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Generic is a specification for a Generic resource
type Generic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Selector GenericSelector `json:"selector"`
	Spec     GenericSpec     `json:"spec"`
	Status   GenericStatus   `json:"status"`
}

// GenericSelector - selector of pods for applying Generic resource
type GenericSelector struct {
	MatchLabels map[string]string `json:"matchLabels"`
}

// GenericSpec is the spec for a Generic resource
type GenericSpec struct {
	Service    string `json:"service"`
	Config GenericSpecConfig `json:"config"`
}

type GenericSpecConfig struct {
	Parameters map[string]string `json:"parameters"`
}

// GenericStatus is the status for a Generic resource
type GenericStatus struct {
	SeenBy int32 `json:"seenBy"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GenericList is a list of Generic resources
type GenericList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Generic `json:"items"`
}
