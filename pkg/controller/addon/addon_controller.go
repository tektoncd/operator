package addon

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	mf "github.com/jcrossley3/manifestival"
	"github.com/prometheus/common/log"
	op "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/controller/setup"
	"golang.org/x/xerrors"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	ctrlLog                   = logf.Log.WithName("ctrl").WithName("tektonaddon")
	errPipelineNotReady       = xerrors.Errorf("tekton-pipelines not ready")
	errAddonVersionUnresolved = xerrors.Errorf("could not resolve to a valid tektonaddon version")
)

// Add creates a new Tekton Addon Controller and adds it to the Manager. The Manager will set fields on the Controller
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
	c, err := controller.New("tektonaddon-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Tekton Addon
	err = c.Watch(&source.Kind{Type: &op.TektonAddon{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watches for secondary resources
	// currently watching only deployments
	err = c.Watch(
		&source.Kind{Type: &appsv1.Deployment{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &op.TektonAddon{},
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

// Reconcile reads that state of the cluster for a TektonAddon object and makes changes based on the state read
// and what is in the TektonAddon.Spec
func (r *ReconcileAddon) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := requestLogger(req, "reconcile")
	log.Info("Reconciling TektonAddon")

	// Fetch the Addon instance
	instance := &op.TektonAddon{}
	err := r.client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("resource has been deleted")
			return reconcile.Result{}, nil
			// TektonAddon components (items in yaml manifest) will be deleted
			// as the owner reference is set, technically we don't have to explicitly delete them
			// if we wan't to delete them manually (if in case of orphaned items)
			// a more complex reconcile logic can be implemented here
			// for example: 1. read manifest, 2. find pipeline-installation,
			// 3. note: can't set owner reference as the TektonAddon resource is not found
			// 4. inject namespace into resources suing target namespace from pipeline
			// 5. delete all resources
			// NT: aug-22-2019
		}
		return reconcile.Result{}, err
	}

	//if no version is specified in spec
	//then, try to set latest available version for the addon in addon spec
	ok, err := r.ensureAddonVersion(instance)

	if !ok {
		return reconcile.Result{Requeue: true}, err
	}

	if isAddOnUpToDate(instance) {
		log.Info("skipping installation, resource already up to date")
		return reconcile.Result{}, nil
	}

	return r.reconcileAddon(req, instance)
}

func (r *ReconcileAddon) ensureAddonVersion(res *op.TektonAddon) (bool, error) {
	version := res.Spec.Version
	if version != "" {
		return true, nil
	}
	version, err := GetLatestVersion(res)
	if err != nil {
		return false, err
	}

	res.Spec.Version = version
	err = r.client.Update(context.TODO(), res)
	if err != nil {
		return false, err
	}

	return true, nil
}

func isAddOnUpToDate(res *op.TektonAddon) bool {
	c := res.Status.Conditions
	if len(c) == 0 {
		return false
	}
	latest := c[0]
	return latest.Version == res.Spec.Version &&
		latest.Code == op.InstalledStatus
}

func (r *ReconcileAddon) reconcileAddon(req reconcile.Request, res *op.TektonAddon) (reconcile.Result, error) {
	log := requestLogger(req, "addon install")

	err := r.updateStatus(res, op.TektonAddonCondition{Code: op.InstallingStatus, Version: res.Spec.Version})
	if err != nil {
		log.Error(err, "failed to set status")
		return reconcile.Result{Requeue: true}, err
	}

	//find the valid clusterwide tekton-pipeline installation
	piplnRes, err := r.pipelineReady()
	if err != nil {
		_ = r.updateStatus(res, op.TektonAddonCondition{
			Code:    op.ErrorStatus,
			Details: err.Error(),
			Version: res.Spec.Version,
		})

		if err == errPipelineNotReady {
			// wait for pipeline status to change
			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}
		// wait longer as pipeline-install not found
		// (tektonpipeline.opeator.tekton.dev instance not available yet)
		return reconcile.Result{RequeueAfter: 2 * time.Minute}, err
	}

	// set the tekton-pipeline installation as the owner for this addon
	err = r.setOwnerReference(res, piplnRes)
	if err != nil {
		_ = r.updateStatus(res, op.TektonAddonCondition{
			Code:    op.ErrorStatus,
			Details: err.Error(),
			Version: res.Spec.Version,
		})
		return reconcile.Result{Requeue: true}, err
	}

	// read and pre-process addon yaml manifest
	manifest, err := r.processPayload(res, piplnRes.Spec.TargetNamespace)
	if err != nil {
		log.Error(err, "failed to create addon manifest")
		_ = r.updateStatus(res, op.TektonAddonCondition{
			Code:    op.ErrorStatus,
			Details: err.Error(),
			Version: res.Spec.Version,
		})
		return reconcile.Result{Requeue: true}, err
	}

	//deploy addon components
	if err := manifest.ApplyAll(); err != nil {
		log.Error(err, "failed to apply release.yaml")
		_ = r.updateStatus(res, op.TektonAddonCondition{
			Code:    op.ErrorStatus,
			Details: err.Error(),
			Version: res.Spec.Version,
		})
		return reconcile.Result{Requeue: true}, err
	}

	log.Info("successfully applied all resources")

	err = r.updateStatus(res, op.TektonAddonCondition{
		Code: op.InstalledStatus, Version: res.Spec.Version})

	// requeue true as isUptodate will be validated in the next reconcile loop
	return reconcile.Result{Requeue: true}, err
}

func (r *ReconcileAddon) processPayload(res *op.TektonAddon, targetNS string) (*mf.Manifest, error) {
	addonPath := getAddonPath(res)
	manifest, err := mf.NewManifest(addonPath, true, r.client)
	if err != nil {
		return nil, err
	}

	// set the currnet addon CRD instance as owner for all items in manifest
	// set targetNamespace of the tekton-pipeline installation as the
	// target namespace for the addon components
	tfs := []mf.Transformer{
		mf.InjectOwner(res),
		mf.InjectNamespace(targetNS),
	}

	if err := manifest.Transform(tfs...); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func getAddonPath(res *op.TektonAddon) string {
	addonDir := getAddonBase(res)
	path := filepath.Join(addonDir, res.Spec.Version)
	return path
}

func GetLatestVersion(res *op.TektonAddon) (string, error) {
	dirName := getAddonBase(res)
	items, err := ioutil.ReadDir(dirName)
	if err != nil || len(items) == 0 {
		return "", errAddonVersionUnresolved
	}

	return items[len(items)-1].Name(), nil
}

func getAddonBase(res *op.TektonAddon) string {
	return filepath.Join("deploy", "resources", "addons", res.Name)
}

func requestLogger(req reconcile.Request, context string) logr.Logger {
	return ctrlLog.WithName(context).WithValues(
		"Request.Namespace", req.Namespace,
		"Request.NamespaceName", req.NamespacedName,
		"Request.Name", req.Name)
}

// updateStatus set the status of res to s and refreshes res to the lastest version
func (r *ReconcileAddon) updateStatus(res *op.TektonAddon, c op.TektonAddonCondition) error {
	res.Status.Conditions = append([]op.TektonAddonCondition{c}, res.Status.Conditions...)

	res.GetObjectMeta()
	if err := r.client.Status().Update(context.TODO(), res); err != nil {
		log.Error(err, "status update failed")
		return err
	}

	return nil
}

func (r *ReconcileAddon) getPipelineRes() (*op.TektonPipeline, error) {
	res := &op.TektonPipeline{}
	namespacedName := types.NamespacedName{
		Name: setup.ClusterCRName,
	}
	err := r.client.Get(context.TODO(), namespacedName, res)
	return res, err
}

func (r *ReconcileAddon) pipelineReady() (*op.TektonPipeline, error) {
	ppln, err := r.getPipelineRes()
	if err != nil {
		return nil, xerrors.Errorf(errPipelineNotReady.Error(), err)
	}
	if ppln.Status.Conditions[0].Code != op.InstalledStatus {
		return nil, errPipelineNotReady
	}
	return ppln, nil
}

func (r *ReconcileAddon) setOwnerReference(res *op.TektonAddon, owner *op.TektonPipeline) error {
	controller := false
	blockOwnerDeletion := true
	res.SetOwnerReferences(
		[]v1.OwnerReference{
			{
				APIVersion:         owner.APIVersion,
				Kind:               owner.Kind,
				Name:               owner.Name,
				UID:                owner.UID,
				Controller:         &controller,
				BlockOwnerDeletion: &blockOwnerDeletion,
			},
		})

	err := r.client.Update(context.TODO(), res)
	if err != nil {
		log.Info("ownerRef", "update", err)
		return err
	}

	return nil
}
