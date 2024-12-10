package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// Reconciler reconciles CertificateSigningRequest objects.
type NetworkPoliciesInstaller struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile performs the main reconciliation logic.
func (r *NetworkPoliciesInstaller) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ns := &corev1.Namespace{}
	if err := r.Get(ctx, request.NamespacedName, ns); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Namespace is gone, stop reconciling network policies")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving Namespace from store: %w", err)
	}

	if ns.DeletionTimestamp != nil {
		log.Info("Namespace is being deleted, stop reconciling network policies")
		return reconcile.Result{}, nil
	}

	// Create a deny-all-egress network policy for the namespace.
	log.Info("Creating deny-all-egress network policy", "Namespace", ns.Name)
	np := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "deny-all-egress", Namespace: ns.Name}}

	_, err := controllerutil.CreateOrPatch(ctx, r.Client, np, func() error {
		np = getDenyAllEgressNetworkPolicy(ns.Name)
		return nil
	})
	return reconcile.Result{}, err
}

// AddToManager adds Reconciler to the given manager.
func (r *NetworkPoliciesInstaller) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("NetworkPolicy installer").
		Watches(&corev1.Namespace{}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(
			predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update),
			predicate.NewPredicateFuncs(func(obj client.Object) bool {
				ns, ok := obj.(*corev1.Namespace)
				if !ok {
					return false
				}
				if len(ns.GetLabels()) < 1 {
					return false
				}
				edgelmLabel, ok := ns.GetLabels()["edgelm.sap.com/product"]
				return ok && edgelmLabel == "edgelm"
			}),
		)).Complete(r)
}

func getDenyAllEgressNetworkPolicy(namespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deny-all-egress",
			Namespace: namespace,
			Labels:    map[string]string{"edgelm.sap.com/product": "edgelm"},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			PodSelector: metav1.LabelSelector{},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{Port: ptr.To(intstr.FromInt32(53)), Protocol: ptr.To(corev1.ProtocolTCP)},
						{Port: ptr.To(intstr.FromInt32(53)), Protocol: ptr.To(corev1.ProtocolUDP)},
						{Port: ptr.To(intstr.FromInt32(8053)), Protocol: ptr.To(corev1.ProtocolTCP)},
						{Port: ptr.To(intstr.FromInt32(8053)), Protocol: ptr.To(corev1.ProtocolUDP)},
					},
				},
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{},
							PodSelector:       &metav1.LabelSelector{},
						},
					},
				},
			},
		},
	}
}
