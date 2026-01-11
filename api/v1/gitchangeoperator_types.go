package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// GitChangeOperatorSpec defines the desired state of GitChangeOperator
type GitChangeOperatorSpec struct {
	// ReplicaCount is the number of replicas for the operator deployment
	// +optional
	ReplicaCount int32 `json:"replicaCount,omitempty"`

	// Image configuration for the operator
	// +optional
	Image ImageConfig `json:"image,omitempty"`

	// Operator configuration
	// +optional
	Operator OperatorConfig `json:"operator,omitempty"`

	// RBAC configuration
	// +optional
	RBAC RBACConfig `json:"rbac,omitempty"`

	// ServiceAccount configuration
	// +optional
	ServiceAccount ServiceAccountConfig `json:"serviceAccount,omitempty"`

	// Metrics configuration
	// +optional
	Metrics MetricsConfig `json:"metrics,omitempty"`

	// CRDs configuration
	// +optional
	CRDs CRDsConfig `json:"crds,omitempty"`

	// Ingress configuration
	// +optional
	Ingress IngressConfig `json:"ingress,omitempty"`

	// Additional values for Helm chart customization
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	AdditionalValues runtime.RawExtension `json:"additionalValues,omitempty"`
}

// ImageConfig defines the container image configuration
type ImageConfig struct {
	// Repository is the container image repository
	// +optional
	Repository string `json:"repository,omitempty"`

	// Tag is the container image tag
	// +optional
	Tag string `json:"tag,omitempty"`

	// PullPolicy is the image pull policy
	// +optional
	PullPolicy string `json:"pullPolicy,omitempty"`
}

// OperatorConfig defines operator runtime configuration
type OperatorConfig struct {
	// LeaderElect enables leader election
	// +optional
	LeaderElect bool `json:"leaderElect,omitempty"`

	// MetricsAddr is the address for metrics endpoint
	// +optional
	MetricsAddr string `json:"metricsAddr,omitempty"`

	// ProbeAddr is the address for health probe endpoint
	// +optional
	ProbeAddr string `json:"probeAddr,omitempty"`
}

// RBACConfig defines RBAC configuration
type RBACConfig struct {
	// Create specifies whether to create RBAC resources
	// +optional
	Create bool `json:"create,omitempty"`
}

// ServiceAccountConfig defines ServiceAccount configuration
type ServiceAccountConfig struct {
	// Create specifies whether to create a ServiceAccount
	// +optional
	Create bool `json:"create,omitempty"`

	// Name is the name of the ServiceAccount to use
	// +optional
	Name string `json:"name,omitempty"`
}

// MetricsConfig defines metrics configuration
type MetricsConfig struct {
	// Enabled specifies whether metrics are enabled
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Service configuration for metrics
	// +optional
	Service MetricsServiceConfig `json:"service,omitempty"`

	// ServiceMonitor configuration
	// +optional
	ServiceMonitor ServiceMonitorConfig `json:"serviceMonitor,omitempty"`
}

// MetricsServiceConfig defines metrics service configuration
type MetricsServiceConfig struct {
	// Type is the Kubernetes service type
	// +optional
	Type string `json:"type,omitempty"`

	// Port is the service port
	// +optional
	Port int32 `json:"port,omitempty"`
}

// ServiceMonitorConfig defines ServiceMonitor configuration
type ServiceMonitorConfig struct {
	// Enabled specifies whether ServiceMonitor is enabled
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Name of the ServiceMonitor resource
	// +optional
	Name string `json:"name,omitempty"`

	// Interval is the scrape interval
	// +optional
	Interval string `json:"interval,omitempty"`

	// ScrapeTimeout is the scrape timeout
	// +optional
	ScrapeTimeout string `json:"scrapeTimeout,omitempty"`

	// Labels to add to the ServiceMonitor
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations to add to the ServiceMonitor
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// CRDsConfig defines CRD installation configuration
type CRDsConfig struct {
	// Install specifies whether to install CRDs
	// +optional
	Install bool `json:"install,omitempty"`
}

// IngressConfig defines Ingress configuration
type IngressConfig struct {
	// Enabled specifies whether Ingress is enabled
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Name of the Ingress resource
	// +optional
	Name string `json:"name,omitempty"`

	// Labels to add to the Ingress
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations to add to the Ingress
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// IngressClassName is the IngressClass to use
	// +optional
	IngressClassName *string `json:"ingressClassName,omitempty"`

	// Hosts is the list of hosts for the Ingress
	// +optional
	Hosts []IngressHost `json:"hosts,omitempty"`

	// TLS configuration
	// +optional
	TLS []IngressTLS `json:"tls,omitempty"`
}

// IngressHost defines an Ingress host configuration
type IngressHost struct {
	// Host name
	Host string `json:"host,omitempty"`

	// Paths for this host
	// +optional
	Paths []IngressPath `json:"paths,omitempty"`
}

// IngressPath defines an Ingress path configuration
type IngressPath struct {
	// Path to match
	Path string `json:"path,omitempty"`

	// PathType (Prefix, Exact, ImplementationSpecific)
	// +optional
	PathType string `json:"pathType,omitempty"`

	// Backend defines the backend service
	Backend IngressBackend `json:"backend,omitempty"`
}

// IngressBackend defines an Ingress backend
type IngressBackend struct {
	// Service backend
	Service IngressServiceBackend `json:"service,omitempty"`
}

// IngressServiceBackend defines a service backend
type IngressServiceBackend struct {
	// Name of the service
	Name string `json:"name,omitempty"`

	// Port of the service
	Port IngressServicePort `json:"port,omitempty"`
}

// IngressServicePort defines a service port
type IngressServicePort struct {
	// Number is the port number
	Number int32 `json:"number,omitempty"`
}

// IngressTLS defines TLS configuration for Ingress
type IngressTLS struct {
	// Hosts covered by this TLS configuration
	// +optional
	Hosts []string `json:"hosts,omitempty"`

	// SecretName containing the TLS certificate
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// GitChangeOperatorStatus defines the observed state of GitChangeOperator
type GitChangeOperatorStatus struct {
	// Phase represents the current phase of the operator deployment
	// +optional
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the operator's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the last generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced

// GitChangeOperator is the Schema for the gitchangeoperators API
type GitChangeOperator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitChangeOperatorSpec   `json:"spec,omitempty"`
	Status GitChangeOperatorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GitChangeOperatorList contains a list of GitChangeOperator
type GitChangeOperatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitChangeOperator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitChangeOperator{}, &GitChangeOperatorList{})
}
