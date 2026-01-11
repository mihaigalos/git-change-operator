package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gitchangeoperatoriov1 "github.com/mihaigalos/git-change-operator/api/v1"
)

const finalizerName = "gitchangeoperator.gco.galos.one/finalizer"

// GitChangeOperatorReconciler reconciles a GitChangeOperator object
// This controller manages the operator's own configuration and resources
type GitChangeOperatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=gco.galos.one,resources=gitchangeoperators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gco.galos.one,resources=gitchangeoperators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gco.galos.one,resources=gitchangeoperators/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

func (r *GitChangeOperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the GitChangeOperator instance
	var gitChangeOperator gitchangeoperatoriov1.GitChangeOperator
	if err := r.Get(ctx, req.NamespacedName, &gitChangeOperator); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciling GitChangeOperator", "name", gitChangeOperator.Name, "namespace", gitChangeOperator.Namespace)

	// Handle deletion
	if !gitChangeOperator.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &gitChangeOperator)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&gitChangeOperator, finalizerName) {
		controllerutil.AddFinalizer(&gitChangeOperator, finalizerName)
		if err := r.Update(ctx, &gitChangeOperator); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile ServiceAccount
	if gitChangeOperator.Spec.ServiceAccount.Create {
		if err := r.reconcileServiceAccount(ctx, &gitChangeOperator); err != nil {
			log.Error(err, "Failed to reconcile ServiceAccount")
			return ctrl.Result{}, err
		}
	}

	// Reconcile RBAC
	if gitChangeOperator.Spec.RBAC.Create {
		if err := r.reconcileClusterRole(ctx, &gitChangeOperator); err != nil {
			log.Error(err, "Failed to reconcile ClusterRole")
			return ctrl.Result{}, err
		}
		if err := r.reconcileClusterRoleBinding(ctx, &gitChangeOperator); err != nil {
			log.Error(err, "Failed to reconcile ClusterRoleBinding")
			return ctrl.Result{}, err
		}
	}

	// Reconcile Metrics Service
	if gitChangeOperator.Spec.Metrics.Enabled {
		if err := r.reconcileMetricsService(ctx, &gitChangeOperator); err != nil {
			log.Error(err, "Failed to reconcile Metrics Service")
			return ctrl.Result{}, err
		}

		// Reconcile ServiceMonitor
		if gitChangeOperator.Spec.Metrics.ServiceMonitor.Enabled {
			if err := r.reconcileServiceMonitor(ctx, &gitChangeOperator); err != nil {
				log.Error(err, "Failed to reconcile ServiceMonitor")
				return ctrl.Result{}, err
			}
		} else {
			// Delete ServiceMonitor if it exists but is disabled
			if err := r.deleteServiceMonitor(ctx, &gitChangeOperator); err != nil {
				log.Error(err, "Failed to delete ServiceMonitor")
				return ctrl.Result{}, err
			}
		}
	} else {
		// Delete both Service and ServiceMonitor if metrics are disabled
		if err := r.deleteMetricsService(ctx, &gitChangeOperator); err != nil {
			log.Error(err, "Failed to delete Metrics Service")
			return ctrl.Result{}, err
		}
		if err := r.deleteServiceMonitor(ctx, &gitChangeOperator); err != nil {
			log.Error(err, "Failed to delete ServiceMonitor")
			return ctrl.Result{}, err
		}
	}

	// Reconcile Ingress
	if gitChangeOperator.Spec.Ingress.Enabled {
		if err := r.reconcileIngress(ctx, &gitChangeOperator); err != nil {
			log.Error(err, "Failed to reconcile Ingress")
			return ctrl.Result{}, err
		}
	} else {
		// Delete Ingress if it exists but is disabled
		if err := r.deleteIngress(ctx, &gitChangeOperator); err != nil {
			log.Error(err, "Failed to delete Ingress")
			return ctrl.Result{}, err
		}
	}

	// Update status
	gitChangeOperator.Status.Phase = "Ready"
	gitChangeOperator.Status.ObservedGeneration = gitChangeOperator.Generation
	if err := r.Status().Update(ctx, &gitChangeOperator); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled GitChangeOperator")
	return ctrl.Result{}, nil
}

func (r *GitChangeOperatorReconciler) handleDeletion(ctx context.Context, gco *gitchangeoperatoriov1.GitChangeOperator) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(gco, finalizerName) {
		// Remove owned resources (they'll be garbage collected due to owner references)
		controllerutil.RemoveFinalizer(gco, finalizerName)
		if err := r.Update(ctx, gco); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *GitChangeOperatorReconciler) reconcileServiceAccount(ctx context.Context, gco *gitchangeoperatoriov1.GitChangeOperator) error {
	saName := gco.Spec.ServiceAccount.Name
	if saName == "" {
		saName = fmt.Sprintf("%s-controller-manager", gco.Name)
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: gco.Namespace,
		},
	}

	if err := controllerutil.SetControllerReference(gco, sa, r.Scheme); err != nil {
		return err
	}

	found := &corev1.ServiceAccount{}
	err := r.Get(ctx, client.ObjectKey{Name: sa.Name, Namespace: sa.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, sa)
	} else if err != nil {
		return err
	}

	// Update if needed
	return r.Update(ctx, sa)
}

func (r *GitChangeOperatorReconciler) reconcileClusterRole(ctx context.Context, gco *gitchangeoperatoriov1.GitChangeOperator) error {
	crName := fmt.Sprintf("%s-manager-role", gco.Name)

	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"gco.galos.one"},
				Resources: []string{"gitcommits", "pullrequests", "gitchangeoperators"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"gco.galos.one"},
				Resources: []string{"gitcommits/finalizers", "pullrequests/finalizers", "gitchangeoperators/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"gco.galos.one"},
				Resources: []string{"gitcommits/status", "pullrequests/status", "gitchangeoperators/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	found := &rbacv1.ClusterRole{}
	err := r.Get(ctx, client.ObjectKey{Name: cr.Name}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, cr)
	} else if err != nil {
		return err
	}

	// Update rules
	found.Rules = cr.Rules
	return r.Update(ctx, found)
}

func (r *GitChangeOperatorReconciler) reconcileClusterRoleBinding(ctx context.Context, gco *gitchangeoperatoriov1.GitChangeOperator) error {
	crbName := fmt.Sprintf("%s-manager-rolebinding", gco.Name)
	saName := gco.Spec.ServiceAccount.Name
	if saName == "" {
		saName = fmt.Sprintf("%s-controller-manager", gco.Name)
	}

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: crbName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     fmt.Sprintf("%s-manager-role", gco.Name),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: gco.Namespace,
			},
		},
	}

	found := &rbacv1.ClusterRoleBinding{}
	err := r.Get(ctx, client.ObjectKey{Name: crb.Name}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, crb)
	} else if err != nil {
		return err
	}

	// Update subjects and roleref
	found.RoleRef = crb.RoleRef
	found.Subjects = crb.Subjects
	return r.Update(ctx, found)
}

func (r *GitChangeOperatorReconciler) reconcileMetricsService(ctx context.Context, gco *gitchangeoperatoriov1.GitChangeOperator) error {
	svcName := fmt.Sprintf("%s-metrics", gco.Name)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: gco.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "git-change-operator",
				"app.kubernetes.io/component": "metrics",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceType(gco.Spec.Metrics.Service.Type),
			Ports: []corev1.ServicePort{
				{
					Name:       "metrics",
					Port:       gco.Spec.Metrics.Service.Port,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"control-plane": "controller-manager",
			},
		},
	}

	if err := controllerutil.SetControllerReference(gco, svc, r.Scheme); err != nil {
		return err
	}

	found := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKey{Name: svc.Name, Namespace: svc.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, svc)
	} else if err != nil {
		return err
	}

	// Update spec
	found.Spec.Type = svc.Spec.Type
	found.Spec.Ports = svc.Spec.Ports
	return r.Update(ctx, found)
}

func (r *GitChangeOperatorReconciler) deleteMetricsService(ctx context.Context, gco *gitchangeoperatoriov1.GitChangeOperator) error {
	svcName := fmt.Sprintf("%s-metrics-service", gco.Name)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: gco.Namespace,
		},
	}

	err := r.Delete(ctx, svc)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func (r *GitChangeOperatorReconciler) reconcileServiceMonitor(ctx context.Context, gco *gitchangeoperatoriov1.GitChangeOperator) error {
	smName := gco.Spec.Metrics.ServiceMonitor.Name
	if smName == "" {
		smName = fmt.Sprintf("%s-controller-manager-metrics-monitor", gco.Name)
	}

	// Build the ServiceMonitor as an unstructured object (monitoring.coreos.com/v1 may not be registered)
	sm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "monitoring.coreos.com/v1",
			"kind":       "ServiceMonitor",
			"metadata": map[string]interface{}{
				"name":      smName,
				"namespace": gco.Namespace,
				"labels":    gco.Spec.Metrics.ServiceMonitor.Labels,
			},
			"spec": map[string]interface{}{
				"endpoints": []interface{}{
					map[string]interface{}{
						"path":            "/metrics",
						"port":            "https",
						"scheme":          "https",
						"bearerTokenFile": "/var/run/secrets/kubernetes.io/serviceaccount/token",
						"tlsConfig": map[string]interface{}{
							"insecureSkipVerify": true,
						},
					},
				},
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"control-plane": "controller-manager",
					},
				},
			},
		},
	}

	// Add annotations if provided
	if len(gco.Spec.Metrics.ServiceMonitor.Annotations) > 0 {
		metadata := sm.Object["metadata"].(map[string]interface{})
		metadata["annotations"] = gco.Spec.Metrics.ServiceMonitor.Annotations
	}

	// Set controller reference
	sm.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "monitoring.coreos.com",
		Version: "v1",
		Kind:    "ServiceMonitor",
	})
	if err := controllerutil.SetControllerReference(gco, sm, r.Scheme); err != nil {
		return err
	}

	// Check if ServiceMonitor exists
	found := &unstructured.Unstructured{}
	found.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "monitoring.coreos.com",
		Version: "v1",
		Kind:    "ServiceMonitor",
	})

	err := r.Get(ctx, types.NamespacedName{Name: smName, Namespace: gco.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, sm)
	} else if err != nil {
		return err
	}

	// Update if exists
	sm.SetResourceVersion(found.GetResourceVersion())
	return r.Update(ctx, sm)
}

func (r *GitChangeOperatorReconciler) deleteServiceMonitor(ctx context.Context, gco *gitchangeoperatoriov1.GitChangeOperator) error {
	smName := gco.Spec.Metrics.ServiceMonitor.Name
	if smName == "" {
		smName = fmt.Sprintf("%s-controller-manager-metrics-monitor", gco.Name)
	}

	sm := &unstructured.Unstructured{}
	sm.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "monitoring.coreos.com",
		Version: "v1",
		Kind:    "ServiceMonitor",
	})
	sm.SetName(smName)
	sm.SetNamespace(gco.Namespace)

	err := r.Delete(ctx, sm)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func (r *GitChangeOperatorReconciler) reconcileIngress(ctx context.Context, gco *gitchangeoperatoriov1.GitChangeOperator) error {
	ingressName := gco.Spec.Ingress.Name
	if ingressName == "" {
		ingressName = fmt.Sprintf("%s-ingress", gco.Name)
	}

	// Build Ingress object
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingressName,
			Namespace:   gco.Namespace,
			Labels:      gco.Spec.Ingress.Labels,
			Annotations: gco.Spec.Ingress.Annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: gco.Spec.Ingress.IngressClassName,
			Rules:            []networkingv1.IngressRule{},
		},
	}

	// Add rules from hosts configuration
	for _, host := range gco.Spec.Ingress.Hosts {
		rule := networkingv1.IngressRule{
			Host: host.Host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{},
				},
			},
		}

		for _, path := range host.Paths {
			pathType := networkingv1.PathTypePrefix
			if path.PathType != "" {
				pathType = networkingv1.PathType(path.PathType)
			}

			rule.HTTP.Paths = append(rule.HTTP.Paths, networkingv1.HTTPIngressPath{
				Path:     path.Path,
				PathType: &pathType,
				Backend: networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{
						Name: path.Backend.Service.Name,
						Port: networkingv1.ServiceBackendPort{
							Number: path.Backend.Service.Port.Number,
						},
					},
				},
			})
		}

		ingress.Spec.Rules = append(ingress.Spec.Rules, rule)
	}

	// Add TLS configuration
	if len(gco.Spec.Ingress.TLS) > 0 {
		ingress.Spec.TLS = []networkingv1.IngressTLS{}
		for _, tls := range gco.Spec.Ingress.TLS {
			ingress.Spec.TLS = append(ingress.Spec.TLS, networkingv1.IngressTLS{
				Hosts:      tls.Hosts,
				SecretName: tls.SecretName,
			})
		}
	}

	// Set controller reference
	if err := controllerutil.SetControllerReference(gco, ingress, r.Scheme); err != nil {
		return err
	}

	// Check if Ingress exists
	found := &networkingv1.Ingress{}
	err := r.Get(ctx, types.NamespacedName{Name: ingressName, Namespace: gco.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, ingress)
	} else if err != nil {
		return err
	}

	// Update if exists
	found.Spec = ingress.Spec
	found.Labels = ingress.Labels
	found.Annotations = ingress.Annotations
	return r.Update(ctx, found)
}

func (r *GitChangeOperatorReconciler) deleteIngress(ctx context.Context, gco *gitchangeoperatoriov1.GitChangeOperator) error {
	ingressName := gco.Spec.Ingress.Name
	if ingressName == "" {
		ingressName = fmt.Sprintf("%s-ingress", gco.Name)
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: gco.Namespace,
		},
	}

	err := r.Delete(ctx, ingress)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GitChangeOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gitchangeoperatoriov1.GitChangeOperator{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
