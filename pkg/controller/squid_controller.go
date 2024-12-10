package controller

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	time "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.wdf.sap.corp/edgelm/network-policy-installer/charts"
)

// Reconciler reconciles CertificateSigningRequest objects.
type SquidInstaller struct {
	client.Client
	kubernetes.ChartApplier
	Scheme *runtime.Scheme
}

// Reconcile performs the main reconciliation logic.
func (r *SquidInstaller) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	config := &corev1.Secret{}
	if err := r.Get(ctx, request.NamespacedName, config); err != nil {
		if apierrors.IsNotFound(err) {
			config.DeletionTimestamp = ptr.To(time.Now())
		} else {
			return reconcile.Result{}, fmt.Errorf("error retrieving Squid config secret from store: %w", err)
		}
	}

	if config.DeletionTimestamp != nil {
		log.Info("Squid config secret is being deleted, deleting squid")
		return reconcile.Result{}, r.Destroy(ctx, config.Namespace, getSquidConfiguration(config))
	}

	// Create a squid proxy for the namespace.
	log.Info("Creating Squid Proxy", "Namespace", config.Namespace)
	return reconcile.Result{}, r.Deploy(ctx, config.Namespace, getSquidConfiguration(config))
}

// AddToManager adds Reconciler to the given manager.
func (r *SquidInstaller) SetupWithManager(mgr manager.Manager) (err error) {
	r.ChartApplier, err = kubernetes.NewChartApplierForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("Squid Proxy installer").
		Watches(&corev1.Secret{}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(
			predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update, predicateutils.Delete),
			predicate.NewPredicateFuncs(func(obj client.Object) bool {
				config, ok := obj.(*corev1.Secret)
				return ok && config.Name == "squid"
			}),
		)).Complete(r)
}

func getSquidConfiguration(secret *corev1.Secret) map[string]interface{} {
	config := make(map[string]interface{})
	if secret == nil || secret.Data == nil {
		return config
	}

	if username, ok := secret.Data["username"]; ok {
		config["username"] = string(username)
	}

	if password, ok := secret.Data["password"]; ok {
		config["password"] = string(password)
	}

	return config
}

func (c *SquidInstaller) Deploy(ctx context.Context, namespace string, values map[string]interface{}) error {
	return c.ApplyFromEmbeddedFS(ctx, charts.ChartSquid, charts.ChartPathSquid, namespace, "squid", kubernetes.Values(values))
}

func (c *SquidInstaller) Destroy(ctx context.Context, namespace string, values map[string]interface{}) error {
	return c.DeleteFromEmbeddedFS(ctx, charts.ChartSquid, charts.ChartPathSquid, namespace, "squid", kubernetes.Values(values))
}
