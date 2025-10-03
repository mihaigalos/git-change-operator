package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RestAPI defines configuration for REST API integration
type RestAPI struct {
	// URL is the REST API endpoint to call
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern="^https?://.*"
	URL string `json:"url"`

	// Method is the HTTP method to use (default: GET)
	// +kubebuilder:validation:Enum=GET;POST;PUT;PATCH;DELETE;HEAD;OPTIONS
	// +kubebuilder:default=GET
	Method string `json:"method,omitempty"`

	// Headers contains HTTP headers to send with the request
	Headers map[string]string `json:"headers,omitempty"`

	// Body is the request body for POST/PUT/PATCH requests
	Body string `json:"body,omitempty"`

	// AuthSecretRef references a secret containing authentication credentials
	AuthSecretRef string `json:"authSecretRef,omitempty"`

	// AuthSecretKey is the key in the auth secret (default: token)
	AuthSecretKey string `json:"authSecretKey,omitempty"`

	// ExpectedStatusCodes defines acceptable HTTP response codes
	// If empty, defaults to [200, 201, 202, 204]
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=10
	ExpectedStatusCodes []int `json:"expectedStatusCodes,omitempty"`

	// MaxStatusCode is the maximum acceptable status code (default: 399)
	// Responses >= this value will prevent the GitCommit/PullRequest from executing
	// +kubebuilder:validation:Minimum=100
	// +kubebuilder:validation:Maximum=599
	// +kubebuilder:default=399
	MaxStatusCode int `json:"maxStatusCode,omitempty"`

	// TimeoutSeconds is the request timeout in seconds (default: 30)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=300
	// +kubebuilder:default=30
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`

	// ResponseParsing configures how to parse and use the JSON response
	ResponseParsing *ResponseParsing `json:"responseParsing,omitempty"`
}

// ResponseParsing defines how to parse JSON responses using CEL expressions
type ResponseParsing struct {
	// Condition is a CEL expression that must evaluate to true for the operation to proceed
	// The JSON response is available as 'response' variable
	// Example: "response.status == 'success' && size(response.data.result) >= 2"
	Condition string `json:"condition,omitempty"`

	// DataExpression is a CEL expression that extracts and transforms data from the response
	// Should return a map with extracted values or a formatted string
	// Example: "{'timestamp': string(response.data.result[0]), 'value': string(response.data.result[1])}"
	DataExpression string `json:"dataExpression,omitempty"`

	// OutputFormat is a CEL expression that formats the final output string
	// The result of DataExpression is available as 'data' variable, current time as 'now'
	// Example: "string(int(now)) + ',' + data.timestamp + ',' + data.value"
	// If empty and DataExpression returns a string, that string is used directly
	OutputFormat string `json:"outputFormat,omitempty"`
}

// RestAPIStatus tracks the status of REST API calls
type RestAPIStatus struct {
	// LastCallTime is when the API was last called
	LastCallTime *metav1.Time `json:"lastCallTime,omitempty"`

	// LastStatusCode is the HTTP status code from the last call
	LastStatusCode int `json:"lastStatusCode,omitempty"`

	// LastResponse is a truncated version of the last response body (max 1024 chars)
	LastResponse string `json:"lastResponse,omitempty"`

	// LastError is the error message from the last failed call
	LastError string `json:"lastError,omitempty"`

	// CallCount is the total number of API calls made
	// +kubebuilder:validation:Type=integer
	CallCount int64 `json:"callCount,omitempty"`

	// SuccessCount is the number of successful API calls
	// +kubebuilder:validation:Type=integer
	SuccessCount int64 `json:"successCount,omitempty"`

	// ConditionMet indicates if the CEL condition expression evaluated to true
	ConditionMet bool `json:"conditionMet,omitempty"`

	// ExtractedData contains the JSON representation of data extracted by the CEL DataExpression
	ExtractedData string `json:"extractedData,omitempty"`

	// FormattedOutput contains the final formatted string produced by the CEL OutputFormat expression
	FormattedOutput string `json:"formattedOutput,omitempty"`
}

type GitCommitSpec struct {
	Repository    string        `json:"repository"`
	Branch        string        `json:"branch"`
	Files         []File        `json:"files,omitempty"`
	ResourceRefs  []ResourceRef `json:"resourceRefs,omitempty"`
	CommitMessage string        `json:"commitMessage"`
	AuthSecretRef string        `json:"authSecretRef"`
	AuthSecretKey string        `json:"authSecretKey,omitempty"`
	Encryption    *Encryption   `json:"encryption,omitempty"`
	RestAPI       *RestAPI      `json:"restAPI,omitempty"`
}

type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`

	// UseRestAPIData indicates this file content should be the formatted REST API response
	// When true, Content is ignored and the file will contain the API response data
	UseRestAPIData bool `json:"useRestAPIData,omitempty"`
}

type ResourceRef struct {
	ApiVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Name       string         `json:"name"`
	Namespace  string         `json:"namespace,omitempty"`
	Strategy   OutputStrategy `json:"strategy"`
}

type OutputStrategy struct {
	Type      OutputType `json:"type"`
	Path      string     `json:"path"`
	WriteMode WriteMode  `json:"writeMode,omitempty"`
	FieldRef  *FieldRef  `json:"fieldRef,omitempty"`
}

type FieldRef struct {
	Key      string `json:"key"`
	FileName string `json:"fileName,omitempty"`
}

type Encryption struct {
	Enabled       bool        `json:"enabled"`
	Recipients    []Recipient `json:"recipients,omitempty"`
	FileExtension string      `json:"fileExtension,omitempty"`
}

type Recipient struct {
	Type      RecipientType `json:"type"`
	Value     string        `json:"value,omitempty"`
	SecretRef *SecretRef    `json:"secretRef,omitempty"`
}

type RecipientType string

const (
	RecipientTypeAge        RecipientType = "age"
	RecipientTypeSSH        RecipientType = "ssh"
	RecipientTypePassphrase RecipientType = "passphrase"
	RecipientTypeYubikey    RecipientType = "yubikey"
)

type SecretRef struct {
	Name string `json:"name"`
	Key  string `json:"key,omitempty"`
}

type OutputType string

const (
	OutputTypeDump        OutputType = "dump"
	OutputTypeFields      OutputType = "fields"
	OutputTypeSingleField OutputType = "single-field"
)

type WriteMode string

const (
	WriteModeOverwrite WriteMode = "overwrite"
	WriteModeAppend    WriteMode = "append"
)

type GitCommitStatus struct {
	CommitSHA     string         `json:"commitSHA,omitempty"`
	Phase         GitCommitPhase `json:"phase,omitempty"`
	Message       string         `json:"message,omitempty"`
	LastSync      *metav1.Time   `json:"lastSync,omitempty"`
	RestAPIStatus *RestAPIStatus `json:"restAPIStatus,omitempty"`
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
