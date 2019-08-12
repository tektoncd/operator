package addon

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/prometheus/common/log"
	"path/filepath"

	mf "github.com/jcrossley3/manifestival"
	op "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	pConfig "github.com/tektoncd/operator/pkg/controller/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	ctrlLog = logf.Log.WithName("ctrl").WithName("config")
)

const (
	DefaultTargetNs = "tekton-pipelines"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Addon Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileAddon{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("addon-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Addon
	err = c.Watch(&source.Kind{Type: &op.Addon{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Addon
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &op.Addon{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileAddon implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileAddon{}

// ReconcileAddon reconciles a Addon object
type ReconcileAddon struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	addons map[string]mf.Manifest
}

// Reconcile reads that state of the cluster for a Addon object and makes changes based on the state read
// and what is in the Addon.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileAddon) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := ctrlLog.WithName("add")
	reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling Addon")

	// Fetch the Addon instance
	instance := &op.Addon{}
	err := r.client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if isAddOnUpToDate(instance) {
		log.Info("skipping installation, resource already up to date")
		return reconcile.Result{}, nil
	}

	return r.reconcileAddon(req, instance)

}

func isAddOnUpToDate(r *op.Addon) bool {
	c := r.Status.Conditions
	if len(c) == 0 {
		return false
	}
	latest := c[0]
	return latest.Version == r.Spec.Version &&
		latest.Code == op.InstalledStatus
}

func (r *ReconcileAddon) reconcileAddon(req reconcile.Request, res *op.Addon) (reconcile.Result, error) {
	log := requestLogger(req, "addon")

	err := r.updateStatus(res, op.AddonCondition{Code: op.InstallingStatus, Version: res.Spec.Version})
	if err != nil {
		log.Error(err, "failed to set status")
		return reconcile.Result{}, err
	}

	addonPath := filepath.Join("deploy", "resources", "addons", res.Name, res.Spec.Version)
	manifest, err := mf.NewManifest(addonPath, true, r.client)
	if err != nil {
		// handle error
	}

	pipeline, err := r.getPipelineRes()
	if err != nil {
		_ = r.updateStatus(res, op.AddonCondition{
			Code:    op.ErrorStatus,
			Details: err.Error(),
			Version: res.Spec.Version,
		})
		return reconcile.Result{}, err
	}
	targetNamespace := pipeline.Namespace
	tfs := []mf.Transformer{
		mf.InjectOwner(res),
		mf.InjectNamespace(targetNamespace),
	}

	if err := manifest.Transform(tfs...); err != nil {
		log.Error(err, "failed to apply manifest transformations")
		// ignoring failure to update
		_ = r.updateStatus(res, op.AddonCondition{
			Code:    op.ErrorStatus,
			Details: err.Error(),
			Version: res.Spec.Version,
		})
		return reconcile.Result{}, err
	}

	if err := manifest.ApplyAll(); err != nil {
		log.Error(err, "failed to apply release.yaml")
		// ignoring failure to update
		_ = r.updateStatus(res, op.AddonCondition{
			Code:    op.ErrorStatus,
			Details: err.Error(),
			Version: res.Spec.Version,
		})
		return reconcile.Result{}, err
	}

	log.Info("successfully applied all resources")

	// NOTE: manifest when updating (not installing) already installed resources
	// modifies the `res` but does not refersh it, hence refresh manually
	if err := r.refreshCR(res); err != nil {
		log.Error(err, "status update failed to refresh object")
		return reconcile.Result{}, err
	}

	err = r.updateStatus(res, op.AddonCondition{
		Code: op.InstalledStatus, Version: res.Spec.Version})
	return reconcile.Result{}, err
}

func requestLogger(req reconcile.Request, context string) logr.Logger {
	return ctrlLog.WithName(context).WithValues(
		"Request.Namespace", req.Namespace,
		"Request.NamespaceName", req.NamespacedName,
		"Request.Name", req.Name)
}

// updateStatus set the status of res to s and refreshes res to the lastest version
func (r *ReconcileAddon) updateStatus(res *op.Addon, c op.AddonCondition) error {

	// NOTE: need to use a deepcopy since Status().Update() seems to reset the
	// APIVersion of the res to "" making the object invalid; may be a mechanism
	// to prevent us from using stale version of the object

	tmp := res.DeepCopy()
	tmp.Status.Conditions = append([]op.AddonCondition{c}, tmp.Status.Conditions...)

	if err := r.client.Status().Update(context.TODO(), tmp); err != nil {
		log.Error(err, "status update failed")
		return err
	}

	if err := r.refreshCR(res); err != nil {
		log.Error(err, "status update failed to refresh object")
		return err
	}
	return nil
}

func (r *ReconcileAddon) refreshCR(res *op.Addon) error {
	objKey := types.NamespacedName{
		Namespace: res.Namespace,
		Name:      res.Name,
	}
	return r.client.Get(context.TODO(), objKey, res)
}

func (r *ReconcileAddon) getPipelineRes() (*op.Config, error) {
	res := &op.Config{}
	namespacedName := types.NamespacedName{
		Namespace: "",
		Name:      pConfig.ClusterCRName,
	}
	err := r.client.Get(context.TODO(), namespacedName, res)

	if err != nil {
		return nil, err
	}
	return res, nil
}
