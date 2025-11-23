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

	// Schedule defines a cron expression for recurring pull requests
	// Examples: "0 2 * * *" (daily at 2 AM), "@hourly", "@daily", "@weekly"
	// When set, the resource will not be deleted and PRs will be created on schedule
	// +kubebuilder:validation:Pattern="^(@(annually|yearly|monthly|weekly|daily|hourly))|(@every (\\d+(ns|us|µs|ms|s|m|h))+)|(((\\d+,)+\\d+|(\\d+([/-])\\d+)|\\d+|\\*) +){4}((\\d+,)+\\d+|(\\d+([/-])\\d+)|\\d+|\\*)$"
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Suspend will suspend execution when set to true. Execution will resume when set to false.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// MaxExecutionHistory specifies how many execution records to keep in status
	// Defaults to 10 if not specified
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=10
	// +optional
	MaxExecutionHistory *int `json:"maxExecutionHistory,omitempty"`
}

// PRExecutionRecord tracks a single execution of a scheduled PullRequest
type PRExecutionRecord struct {
	// ExecutionTime is when the PR was executed
	ExecutionTime metav1.Time `json:"executionTime"`

	// PullRequestNumber is the resulting PR number
	PullRequestNumber int `json:"pullRequestNumber,omitempty"`

	// PullRequestURL is the URL of the created PR
	PullRequestURL string `json:"pullRequestURL,omitempty"`

	// Phase indicates the result of this execution
	Phase PullRequestPhase `json:"phase"`

	// Message contains any error or status message
	Message string `json:"message,omitempty"`
}

type PullRequestStatus struct {
	PullRequestNumber int              `json:"pullRequestNumber,omitempty"`
	PullRequestURL    string           `json:"pullRequestURL,omitempty"`
	Phase             PullRequestPhase `json:"phase,omitempty"`
	Message           string           `json:"message,omitempty"`
	LastSync          *metav1.Time     `json:"lastSync,omitempty"`
	RestAPIStatuses   []RestAPIStatus  `json:"restAPIStatuses,omitempty"`

	// LastScheduledTime is when the resource was last scheduled for execution
	LastScheduledTime *metav1.Time `json:"lastScheduledTime,omitempty"`

	// NextScheduledTime is when the resource will be executed next
	NextScheduledTime *metav1.Time `json:"nextScheduledTime,omitempty"`

	// ExecutionHistory keeps track of the last N executions (configurable via spec.maxExecutionHistory)
	ExecutionHistory []PRExecutionRecord `json:"executionHistory,omitempty"`
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
