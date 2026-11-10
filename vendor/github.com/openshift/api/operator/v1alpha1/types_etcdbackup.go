package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//
// # EtcdBackup provides configuration options and status for a one-time backup attempt of the etcd cluster.
// # When an EtcdBackup is deleted the files created by it will be deleted as well, as long as the storage backend is still accessible.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=etcdbackups,scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Storage Type,JSONPath=.spec.storage.type,type=string,description="Type of storage used for the backup"
// +kubebuilder:printcolumn:name=Status,JSONPath=.status.conditions[?(@.status=="True")].type,type=string,description="Status of the EtcdBackup"
// +kubebuilder:printcolumn:name=Completed At,JSONPath=.status.conditions[?(@.type=="Completed")].lastTransitionTime,type=date,description="Completion time of the EtcdBackup"
// +kubebuilder:printcolumn:name=Age,JSONPath=.metadata.creationTimestamp,type=date,description="Age of the EtcdBackup"
// +kubebuilder:validation:XValidation:rule="self.metadata.name.size() <= 63",message="name cannot exceed 63 characters"
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2952
// +openshift:file-pattern=cvoRunLevel=0000_10,operatorName=etcd,operatorOrdering=01
// +openshift:enable:FeatureGate=AutomatedEtcdBackup
type EtcdBackup struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +required
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable"
	// +required
	Spec EtcdBackupSpec `json:"spec,omitzero"`
	// status holds observed values from the cluster. They may not be overridden.
	// +optional
	Status EtcdBackupStatus `json:"status,omitzero"`
}

type EtcdBackupSpec struct {
	// assets specifies the type of files to be backed up.
	// If Snapshot is present, the etcd database will be snapshotted and saved to the storage backend.
	// If StaticKubeResources is present, static pod manifests for the control plane will be bundled and saved to the storage backend. If etcd is encrypted, this will include the encryption keys.
	// When specified, must contain between 1 and 2 entries, with at most 1 of each type.
	// When omitted, all assets types will be backed up.
	// This field is optional.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=2
	// +listType=set
	// +optional
	Assets []EtcdBackupAssetType `json:"assets,omitempty"`

	// nodeSelector specifies which control plane nodes to select from for running the backup job. Only one backup job is run per EtcdBackup.
	// The default node-role.kubernetes.io/control-plane label will always be added on top of any labels set on nodeSelector.
	// If no nodes are matched, then the backup will be marked failed.
	// For Local storage type, this may be used to target a specific node to take and store the backup.
	// For PVC storage type, this may be used to control where the backup is taken from. If multiple nodes are selected, the scheduler is responsible for placing the backup job based on volume topology and any other scheduling constraints.
	// When specified, must contain between 1 and 10 entries.
	// This field is optional.
	// +kubebuilder:validation:MinProperties=1
	// +kubebuilder:validation:MaxProperties=10
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// storage specifies the location where etcd backup files will be saved.
	// This field is required.
	// +required
	Storage EtcdBackupStorage `json:"storage,omitzero"`

	// --- TOMBSTONE ---
	// pvcName was the name of a PersistentVolumeClaim used to store the backup.
	// The field has been replaced by the storage field, which supports multiple types of storage backends including PVC.
	// The field name is reserved to prevent reuse.
	//
	// +optional
	// PVCName string `json:"pvcName"`
}

// +kubebuilder:validation:Enum=Snapshot;StaticKubeResources
type EtcdBackupAssetType string

const (
	// EtcdBackupAssetTypeSnapshot is used to request an etcd database snapshot.
	EtcdBackupAssetTypeSnapshot EtcdBackupAssetType = "Snapshot"
	// EtcdBackupAssetTypeStaticKubeResources is used to request a static pod manifests bundle.
	EtcdBackupAssetTypeStaticKubeResources EtcdBackupAssetType = "StaticKubeResources"
)

// +kubebuilder:validation:XValidation:rule="self.type == 'PVC' ? has(self.pvc) : !has(self.pvc)",message="pvc is required when type is PVC, and forbidden otherwise"
// +kubebuilder:validation:XValidation:rule="self.type == 'Local' ? true : !has(self.local)",message="local is optional when type is Local, and forbidden otherwise"
// +union
type EtcdBackupStorage struct {
	// type of storage backend to use for storing the etcd backup.
	// Allowed values are Local and PVC.
	// When set to Local, the backup will be saved to a host path directory on the control plane node it was taken on.
	// When set to PVC, the backup will be saved to a volume bound by a PersistentVolumeClaim.
	// This field is required.
	// +kubebuilder:validation:Enum=PVC;Local
	// +required
	// +unionDiscriminator
	Type EtcdBackupStorageType `json:"type,omitempty"`

	// pvc specifies the PersistentVolumeClaim (PVC) which binds a PersistentVolume where the etcd backup file will be saved.
	// The PVC must always be created in the "openshift-etcd" namespace.
	// This field is required when the storage type is "PVC", and forbidden otherwise.
	// +optional
	// +unionMember
	PVC EtcdBackupStoragePvc `json:"pvc,omitzero"`

	// local specifies a host path directory on the control plane node where the etcd backup file will be saved.
	// This field is optional when storage type is "Local", and forbidden otherwise.
	// +optional
	// +unionMember
	Local EtcdBackupStorageLocal `json:"local,omitzero"`
}

// EtcdBackupStorageType is an enum of the supported storage backends for backup files
type EtcdBackupStorageType string

const (
	// EtcdBackupStorageTypePVC configures backups to be stored on a PersistenVolumeClaim.
	EtcdBackupStorageTypePVC EtcdBackupStorageType = "PVC"
	// EtcdBackupStorageTypePVC configures backups to be stored locally on control plane nodes.
	EtcdBackupStorageTypeLocal EtcdBackupStorageType = "Local"
)

type EtcdBackupStoragePvc struct {
	// name is a reference to a PVC in the "openshift-etcd" namespace where the etcd backup file will be saved.
	// name must be between 1 and 253 characters and conform to RFC 1123 subdomain format:
	// lowercase alphanumeric characters, '-' or '.', starting and ending with alphanumeric characters.
	// This field is required.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="name must be a lowercase RFC 1123 subdomain consisting of lowercase alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character"
	// +required
	Name string `json:"name,omitempty"`

	// path is a directory on the volume where the etcd backup file will be saved.
	// When present, path must be an absolute filepath between 1 and 2048 characters containing only alphanumeric characters, '/', '.', '_', or '-', starting with a '/'.
	// This field is optional.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:XValidation:rule=`self.matches('^/[a-zA-Z0-9/._-]+$')`,message="path must be a valid absolute filepath containing only alphanumeric characters, '/', '.', '_', or '-', starting with a '/'"
	// +optional
	Path string `json:"path,omitempty"`
}

// +kubebuilder:validation:MinProperties=1
type EtcdBackupStorageLocal struct {
	// hostPath is a local directory on the control plane node where the etcd backup file will be saved.
	// hostPath must be an absolute filepath between 1 and 2048 characters containing only alphanumeric characters, '/', '.', '_', or '-', starting with a '/'.
	// When omitted, hostPath defaults to "/etc/kubernetes/cluster-backup".
	// This field is optional.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:XValidation:rule=`self.matches('^/[a-zA-Z0-9/._-]+$')`,message="hostPath must be a valid absolute filepath containing only alphanumeric characters, '/', '.', '_', or '-', starting with a '/'"
	// +optional
	HostPath string `json:"hostPath,omitempty"`
}

// +kubebuilder:validation:MinProperties=1
type EtcdBackupStatus struct {
	// conditions provide details on the status of the etcd backup job.
	// Supported conditions include Running, Completed, and Failed.
	// If Running is true, the backup job is currently active.
	// If Completed is true, the backup job succeeded and has saved backup files to its storage location.
	// If Failed is true, the backup job failed and did not create valid backup files.
	//
	// When specified, must contain between 1 and 15 entries, with no more than 1 of each type.
	// This field is optional.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=15
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// jobName is a reference to the Job created for the backup. It always runs in the openshift-etcd namespace.
	// Must be between 1 and 253 characters and conform to RFC 1123 subdomain format:
	// lowercase alphanumeric characters, '-' or '.', starting and ending with alphanumeric characters.
	// This field is optional.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="jobName must be a lowercase RFC 1123 subdomain consisting of lowercase alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character"
	// +optional
	JobName string `json:"jobName,omitempty"`

	// nodeName is used for Local EtcdBackups to bind the control plane node where the snapshot will be taken.
	// It is set by the cluster-etcd-operator before the Local backup runs to ensure the file location is known for garbage collection.
	// Must be between 1 and 253 characters and conform to RFC 1123 subdomain format:
	// lowercase alphanumeric characters, '-' or '.', starting and ending with alphanumeric characters.
	// This field is optional.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="nodeName must be a lowercase RFC 1123 subdomain consisting of lowercase alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character"
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// files tracks the path and size of files generated by the etcd backup.
	// Includes both etcd snapshots and static manifests.
	// When specified, must contain between 1 and 10 entries.
	// This field is optional.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=10
	// +listType=map
	// +listMapKey=path
	// +optional
	Files []EtcdBackupFile `json:"files,omitempty"`

	// --- TOMBSTONE ---
	// backupJob was a reference to the Job that executes the backup.
	// The field has been replaced by jobName.
	// The field name is reserved to prevent reuse.
	//
	// +optional
	// BackupJob *BackupJobReference `json:"backupJob"`
}

type EtcdBackupFile struct {
	// assetType specifies the type of the backup file.
	// If set to Snapshot, the file is an etcd database snapshot.
	// If set to StaticKubeResources, the file is a bundle of static pod manifests for the control plane. If etcd is encrypted, this will include the encryption keys.
	// +required
	AssetType EtcdBackupAssetType `json:"assetType,omitempty"`

	// path to the backup file on the storage backend.
	// Must be an absolute filepath between 1 and 4096 characters containing only alphanumeric characters, '/', '.', '_', or '-', starting with a '/'.
	// This field is required.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	// +kubebuilder:validation:XValidation:rule=`self.matches('^/[a-zA-Z0-9/._-]+$')`,message="path must be a valid absolute filepath"
	// +required
	Path string `json:"path,omitempty"`

	// sizeBytes is the size of the backup file on the storage backend in bytes.
	// If omitted, then the file specified by path is an empty file.
	// When specified, must be between 1 and 1000000000000 (1 TB).
	// This field is optional.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000000000000
	// +optional
	SizeBytes int64 `json:"sizeBytes,omitempty"`
}

// --- TOMBSTONE ---
// BackupJobReference was a reference to the batch/v1 Job created to run the etcd backup
// The type has been replaced by the jobName string field on EtcdBackupStatus.
// The type name is reserved to prevent reuse.
//
// type BackupJobReference struct {
// 	namespace is the namespace of the Job.
// 	this is always expected to be "openshift-etcd" since the user provided PVC
// 	is also required to be in "openshift-etcd"
// 	+required
// 	+kubebuilder:validation:Pattern:=`^openshift-etcd$`
// 	Namespace string `json:"namespace"`
//
// 	name is the name of the Job.
// 	+required
// 	Name string `json:"name"`
// }

// BackupConditionType enumerates the Condition types added to EtcdBackupStatus at different points in its lifecycle
type BackupConditionType string

var (
	// BackupRunning means the backup job has started.
	BackupRunning BackupConditionType = "Running"
	// BackupCompleted means the backup completed successfully.
	BackupCompleted BackupConditionType = "Completed"
	// BackupFailed means the backup failed.
	BackupFailed BackupConditionType = "Failed"
)

// BackupConditionReason enumerates the Condition reasons associated with BackupConditionTypes
type BackupConditionReason string

var (
	// BackupReasonJobStarted means the backup job is currently running.
	BackupReasonJobStarted BackupConditionReason = "JobStarted"

	// BackupReasonJobCompleted means the backup job completed successfully.
	BackupReasonJobCompleted BackupConditionReason = "JobCompleted"

	// BackupReasonSetupFailed means the backup during setup, and no files were created.
	// Backup may have failed to schedule, failed to validate storage, or any error that occurred before the backup process began.
	// Additional details on the failure will be provided in the condition message.
	BackupReasonSetupFailed BackupConditionReason = "SetupFailed"
	// BackupReasonJobFailed means the backup job failed after the backup process began.
	BackupReasonJobFailed BackupConditionReason = "JobFailed"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EtcdBackupList is a collection of items
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type EtcdBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []EtcdBackup `json:"items"`
}
