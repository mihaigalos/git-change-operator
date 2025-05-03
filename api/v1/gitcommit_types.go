package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GitCommitSpec struct {
	Repository    string `json:"repository"`
	Branch        string `json:"branch"`
	Files         []File `json:"files"`
	CommitMessage string `json:"commitMessage"`
	AuthSecretRef string `json:"authSecretRef"`
	AuthSecretKey string `json:"authSecretKey,omitempty"`
}

type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type GitCommitStatus struct {
	CommitSHA string         `json:"commitSHA,omitempty"`
	Phase     GitCommitPhase `json:"phase,omitempty"`
	Message   string         `json:"message,omitempty"`
	LastSync  *metav1.Time   `json:"lastSync,omitempty"`
}

type GitCommitPhase string

const (
	GitCommitPhasePending   GitCommitPhase = "Pending"
	GitCommitPhaseRunning   GitCommitPhase = "Running"
	GitCommitPhaseCommitted GitCommitPhase = "Committed"
	GitCommitPhaseFailed    GitCommitPhase = "Failed"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Repository",type="string",JSONPath=".spec.repository"
//+kubebuilder:printcolumn:name="Branch",type="string",JSONPath=".spec.branch"
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
//+kubebuilder:printcolumn:name="CommitSHA",type="string",JSONPath=".status.commitSHA"

type GitCommit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitCommitSpec   `json:"spec,omitempty"`
	Status GitCommitStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type GitCommitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitCommit `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitCommit{}, &GitCommitList{})
}
