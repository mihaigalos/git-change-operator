package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PullRequestSpec struct {
	Repository string `json:"repository"`
	BaseBranch string `json:"baseBranch"`
	HeadBranch string `json:"headBranch"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Title         string        `json:"title"`
	Body          string        `json:"body,omitempty"`
	Files         []File        `json:"files,omitempty"`
	ResourceRefs  []ResourceRef `json:"resourceRefs,omitempty"`
	AuthSecretRef string        `json:"authSecretRef"`
	AuthSecretKey string        `json:"authSecretKey,omitempty"`
	Encryption    *Encryption   `json:"encryption,omitempty"`
	RestAPIs      []RestAPI     `json:"restAPIs,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=43200
	TTLMinutes *int `json:"ttlMinutes,omitempty"`
}

type PullRequestStatus struct {
	PullRequestNumber int              `json:"pullRequestNumber,omitempty"`
	PullRequestURL    string           `json:"pullRequestURL,omitempty"`
	Phase             PullRequestPhase `json:"phase,omitempty"`
	Message           string           `json:"message,omitempty"`
	LastSync          *metav1.Time     `json:"lastSync,omitempty"`
	RestAPIStatuses   []RestAPIStatus  `json:"restAPIStatuses,omitempty"`
}

type PullRequestPhase string

const (
	PullRequestPhasePending PullRequestPhase = "Pending"
	PullRequestPhaseRunning PullRequestPhase = "Running"
	PullRequestPhaseCreated PullRequestPhase = "Created"
	PullRequestPhaseFailed  PullRequestPhase = "Failed"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Repository",type="string",JSONPath=".spec.repository"
//+kubebuilder:printcolumn:name="Title",type="string",JSONPath=".spec.title"
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
//+kubebuilder:printcolumn:name="PR Number",type="integer",JSONPath=".status.pullRequestNumber"

type PullRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PullRequestSpec   `json:"spec,omitempty"`
	Status PullRequestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type PullRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PullRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PullRequest{}, &PullRequestList{})
}
