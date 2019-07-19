package config

import (
	"context"
	"flag"
	"path/filepath"

	"github.com/go-logr/logr"
	mf "github.com/jcrossley3/manifestival"
	"github.com/operator-framework/operator-sdk/pkg/predicate"
	"github.com/prometheus/common/log"
	op "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

const (
	ClusterCRName   = "cluster"
	DefaultTargetNs = "openshift-pipelines"

	// Name of the pipeline controller deployment
	PipelineControllerName = "tekton-pipelines-controller"

	// Name of the pipeline webhook deployment
	PipelineWebhookName = "tekton-pipelines-webhook"
)

var (
	tektonVersion   = "v0.4.0"
	resourceWatched string
	resourceDir     string
	targetNamespace string
	noAutoInstall   bool
	recursive       bool
	ctrlLog         = logf.Log.WithName("ctrl").WithName("config")
)

func init() {
	flag.StringVar(
		&resourceWatched, "watch-resource", ClusterCRName,
		"cluster-wide resource that operator honours, default: "+ClusterCRName)

	flag.StringVar(
		&targetNamespace, "target-namespace", DefaultTargetNs,
		"Namespace where pipeline will be installed default: "+DefaultTargetNs)

	defaultResDir := filepath.Join("deploy", "resources", tektonVersion)
	flag.StringVar(
		&resourceDir, "resource-dir", defaultResDir,
		"Path to resource manifests, default: "+defaultResDir)

	flag.BoolVar(
		&noAutoInstall, "no-auto-install", false,
		"Do not automatically install tekton pipelines, default: false")

	flag.BoolVar(
		&recursive, "recursive", false,
		"If enabled apply manifest file in resource directory recursively")

	ctrlLog.Info("configuration",
		"resource-watched", resourceWatched,
		"targetNamespace", targetNamespace,
		"no-auto-install", noAutoInstall,
	)
}

// Add creates a new Config Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	m, err := mf.NewManifest(resourceDir, recursive, mgr.GetClient())
	if err != nil {
		return err
	}
	return add(mgr, newReconciler(mgr, m))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, m mf.Manifest) reconcile.Reconciler {
	return &ReconcileConfig{
		client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		manifest: m,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	log := ctrlLog.WithName("add")
	// Create a new controller
	c, err := controller.New("config-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Config
	log.Info("Watching operator config CR")
	err = c.Watch(
		&source.Kind{Type: &op.Config{}},
		&handler.EnqueueRequestForObject{},
		predicate.GenerationChangedPredicate{},
	)
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &appsv1.Deployment{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &op.Config{},
		})
	if err != nil {
		return err
	}

	if noAutoInstall {
		return nil
	}

	if err := createCR(mgr.GetClient()); err != nil {
		log.Error(err, "creation of config resource failed")
		return err
	}
	return nil
}

// blank assignment to verify that ReconcileConfig implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileConfig{}

// ReconcileConfig reconciles a Config object
type ReconcileConfig struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client   client.Client
	scheme   *runtime.Scheme
	manifest mf.Manifest
}

// Reconcile reads that state of the cluster for a Config object and makes changes based on the state read
// and what is in the Config.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileConfig) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := requestLogger(req, "reconcile")

	log.Info("reconciling config change")

	if ignoreRequest(req) {
		log.Info("ignoring event of resource watched that is not cluster-wide")
		return reconcile.Result{}, nil
	}

	res := &op.Config{}
	err := r.client.Get(context.TODO(), req.NamespacedName, res)

	// ignore all resources except the `resourceWatched`
	if req.Name != resourceWatched {
		log.Info("ignoring incorrect object")

		// handle resources that are not interesting as error
		if !errors.IsNotFound(err) {
			r.markInvalidResource(res)
		}
		return reconcile.Result{}, nil
	}

	// handle deletion of resource
	if errors.IsNotFound(err) {
		// User deleted the cluster resource so delete the pipeine resources
		log.Info("resource has been deleted")
		return r.reconcileDeletion(req, res)
	}

	// Error reading the object - requeue the request.
	if err != nil {
		log.Error(err, "requeueing event since there was an error reading object")
		return reconcile.Result{}, err
	}

	if isUpToDate(res) {
		log.Info("skipping installation, resource already up to date")
		return reconcile.Result{}, nil
	}

	log.Info("installing pipelines", "path", resourceDir)

	return r.reconcileInstall(req, res)

}

func (r *ReconcileConfig) reconcileInstall(req reconcile.Request, res *op.Config) (reconcile.Result, error) {
	log := requestLogger(req, "install")

	err := r.updateStatus(res, op.ConfigCondition{Code: op.InstallingStatus, Version: tektonVersion})
	if err != nil {
		log.Error(err, "failed to set status")
		return reconcile.Result{}, err
	}

	tfs := []mf.Transformer{
		mf.InjectOwner(res),
		mf.InjectNamespace(res.Spec.TargetNamespace),
	}

	if err := r.manifest.Transform(tfs...); err != nil {
		log.Error(err, "failed to apply manifest transformations")
		// ignoring failure to update
		_ = r.updateStatus(res, op.ConfigCondition{
			Code:    op.ErrorStatus,
			Details: err.Error(),
			Version: tektonVersion})
		return reconcile.Result{}, err
	}

	if err := r.manifest.ApplyAll(); err != nil {
		log.Error(err, "failed to apply release.yaml")
		// ignoring failure to update
		_ = r.updateStatus(res, op.ConfigCondition{
			Code:    op.ErrorStatus,
			Details: err.Error(),
			Version: tektonVersion})
		return reconcile.Result{}, err
	}
	log.Info("successfully applied all resources")

	// NOTE: manifest when updating (not installing) already installed resources
	// modifies the `res` but does not refersh it, hence refresh manually
	if err := r.refreshCR(res); err != nil {
		log.Error(err, "status update failed to refresh object")
		return reconcile.Result{}, err
	}

	err = r.updateStatus(res, op.ConfigCondition{
		Code: op.InstalledStatus, Version: tektonVersion})
	return reconcile.Result{}, err
}

func (r *ReconcileConfig) reconcileDeletion(req reconcile.Request, res *op.Config) (reconcile.Result, error) {
	log := requestLogger(req, "delete")

	log.Info("deleting pipeline resources")

	// Requested object not found, could have been deleted after reconcile request.
	// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
	propPolicy := client.PropagationPolicy(metav1.DeletePropagationForeground)

	if err := r.manifest.DeleteAll(propPolicy); err != nil {
		log.Error(err, "failed to delete pipeline resources")
		return reconcile.Result{}, err
	}

	// Return and don't requeue
	return reconcile.Result{}, nil

}

// markInvalidResource sets the status of resourse as invalid
func (r *ReconcileConfig) markInvalidResource(res *op.Config) {
	err := r.updateStatus(res,
		op.ConfigCondition{
			Code:    op.ErrorStatus,
			Details: "metadata.name must be " + resourceWatched,
			Version: "unknown"})
	if err != nil {
		ctrlLog.Info("failed to update status as invalid")
	}
}

// updateStatus set the status of res to s and refreshes res to the lastest version
func (r *ReconcileConfig) updateStatus(res *op.Config, c op.ConfigCondition) error {

	// NOTE: need to use a deepcopy since Status().Update() seems to reset the
	// APIVersion of the res to "" making the object invalid; may be a mechanism
	// to prevent us from using stale version of the object

	tmp := res.DeepCopy()
	tmp.Status.Conditions = append([]op.ConfigCondition{c}, tmp.Status.Conditions...)

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

func (r *ReconcileConfig) refreshCR(res *op.Config) error {
	objKey := types.NamespacedName{
		Namespace: res.Namespace,
		Name:      res.Name,
	}
	return r.client.Get(context.TODO(), objKey, res)
}

func createCR(c client.Client) error {
	log := ctrlLog.WithName("create-cr").WithValues("name", resourceWatched)
	log.Info("creating a clusterwide resource of config crd")

	cr := &op.Config{
		ObjectMeta: metav1.ObjectMeta{Name: resourceWatched},
		Spec:       op.ConfigSpec{TargetNamespace: targetNamespace},
	}

	err := c.Create(context.TODO(), cr)
	if errors.IsAlreadyExists(err) {
		log.Info("skipped creation", "reason", "resoure already exists")
		return nil
	}

	return err
}

func isUpToDate(r *op.Config) bool {
	c := r.Status.Conditions
	if len(c) == 0 {
		return false
	}

	latest := c[0]
	return latest.Version == tektonVersion &&
		latest.Code == op.InstalledStatus
}

func requestLogger(req reconcile.Request, context string) logr.Logger {
	return ctrlLog.WithName(context).WithValues(
		"Request.Namespace", req.Namespace,
		"Request.NamespaceName", req.NamespacedName,
		"Request.Name", req.Name)
}

func ignoreRequest(req reconcile.Request) bool {
	// NOTE: only interested in the clusterwide events of the resource watched
	// Otherwise, when release.yaml is applied and onwer-ref is set to the
	// clusterwide resource, events are generated for "targetNamespace/resourceWatched"
	// and as the GET call for that resource fails, the pipeline resources get
	// deleted. this filter prevents that from happening
	return req.Name == resourceWatched && req.Namespace != ""
}
