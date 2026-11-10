package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//
// # EtcdBackupPolicy sets an automated schedule for taking backups of the etcd cluster
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=etcdbackuppolicies,scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Storage Type,JSONPath=.spec.storage.type,type=string,description="Type of storage used for the backup"
// +kubebuilder:printcolumn:name=Schedule,JSONPath=.spec.schedule,type=string,description="Cron schedule for executing backups"
// +kubebuilder:printcolumn:name=TimeZone,JSONPath=.spec.timeZone,type=string,description="Cron time zone for executing backups"
// +kubebuilder:printcolumn:name=Status,JSONPath=.status.conditions[?(@.type=="Scheduled")].reason,type=string,description="Status of the EtcdBackupPolicy"
// +kubebuilder:printcolumn:name=Age,JSONPath=.metadata.creationTimestamp,type=date,description="Age of the EtcdBackupPolicy"
// +kubebuilder:validation:XValidation:rule="self.metadata.name.size() <= 52",message="name cannot exceed 52 characters"
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/2952
// +openshift:file-pattern=cvoRunLevel=0000_10,operatorName=etcd,operatorOrdering=01
// +openshift:enable:FeatureGate=AutomatedEtcdBackup
type EtcdBackupPolicy struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +required
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	// +required
	Spec EtcdBackupPolicySpec `json:"spec,omitzero"`
	// status holds observed values from the cluster. They may not be overridden.
	// +optional
	Status EtcdBackupPolicyStatus `json:"status,omitzero"`
}

type EtcdBackupPolicySpec struct {
	// schedule sets the backup schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
	// The value must be one of the following forms:
	// a predefined macro "@yearly", "@annually", "@monthly", "@weekly", "@daily" or "@hourly";
	// an "@every <duration>" expression where duration is a sequence of decimal numbers each with
	// a unit of "s", "m" or "h" (for example "@every 2h" or "@every 1h30m");
	// or a standard cron expression of five space separated fields (minute, hour, day-of-month,
	// month and day-of-week), where each field may only contain digits, the names used for months
	// and days of the week, and the cron special characters "*", "/", "-" and ",".
	// This validation only checks the overall shape of the value; the individual field values are
	// validated when the schedule is parsed by the controller, which reports parsing failures via
	// the schedule's status condition.
	// Must be between 1 and 1024 characters long.
	// This field is required.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	// +kubebuilder:validation:XValidation:rule=`self.matches("^(@(annually|yearly|monthly|weekly|daily|hourly)|@every ([0-9]+([.][0-9]+)?(s|m|h))+|(([-0-9*/,A-Za-z]+ ){4}[-0-9*/,A-Za-z]+))$")`,message="must be a valid cron expression, or macro"
	// +required
	Schedule string `json:"schedule,omitempty"`

	// timeZone is an IANA time zone database name used to interpret the schedule, see https://en.wikipedia.org/wiki/List_of_tz_database_time_zones.
	// The accepted values are the same as the timeZone field of a CronJob.
	// If not specified, this will default to the time zone of the cluster.
	// Must be between 1 and 64 characters long.
	// This field is optional.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +optional
	TimeZone string `json:"timeZone,omitempty"`

	// assets specifies the type of files to be backed up.
	// If Snapshot is present, the etcd database will be snapshotted and saved to the storage backend.
	// If StaticKubeResources is present, static pod manifests for the control plane will be bundled and saved to the storage backend. If etcd is encrypted, this will include the encryption keys.
	// This field is copied to EtcdBackups created by this policy.
	// When specified, must contain between 1 and 2 entries, with at most 1 of each type.
	// When omitted, all assets types will be backed up.
	// This field is optional.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=2
	// +listType=set
	// +optional
	Assets []EtcdBackupAssetType `json:"assets,omitempty"`

	// nodeSelector specifies which control plane nodes to select from for running backup jobs.
	// The default node-role.kubernetes.io/control-plane label will always be added on top of any labels set on nodeSelector.
	// If no nodes are matched, then no EtcdBackups will be created.
	// For Local storage type, this may be used to target a specific node to take and store the backup. If distribution is AllNodes, then backups are taken on all selected nodes. If distribution is SingleNode, then backups will round robin selected nodes in lexicographical order starting from the last created backup for the policy.
	// For PVC storage type, this may be used to control where the backup is taken from. If multiple nodes are selected, the scheduler is responsible for placing the backup job based on volume topology and any other scheduling constraints.
	// EtcdBackup nodeSelector is derived from this field.
	// When specified, must contain between 1 and 10 entries.
	// This field is optional.
	// +kubebuilder:validation:MinProperties=1
	// +kubebuilder:validation:MaxProperties=10
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// storage specifies the location where etcd backup files will be saved.
	// EtcdBackup storage is derived from this field.
	// In rare cases, where storage type is changed from PVC to Local with AllNodes distribution on an existing policy, old backups on the PVC may need to be manually pruned. Make sure to check that new valid backups exist before deleting old ones.
	// This field is required.
	// +required
	Storage EtcdBackupPolicyStorage `json:"storage,omitzero"`

	// retentionRules defines the policy for retaining and deleting completed backups.
	// Completed backups are deleted from the oldest first until all rules are satisfied.
	// If no rules are specified then backups created by this policy will not be automatically deleted.
	// When an EtcdBackup is deleted the files created by it will be deleted as well, as long as the storage backend is still accessible.
	// For Local storage with distribution AllNodes, retention is calculated for EtcdBackups grouped by Node (e.g. MaxQuantity 2 results in 2 backups per node, MaxTotalSizeGiB 5 results in 5GiB of backups per node). In all other cases retention is calculated across all EtcdBackups belonging to the policy.
	// When specified, must contain between 1 and 2 entries.
	// Must not have more than 1 entry of each rule type.
	// This field is optional.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=2
	// +listType=map
	// +listMapKey=type
	// +optional
	RetentionRules []EtcdBackupPolicyRetentionRule `json:"retentionRules,omitempty"`

	// failedBackupsHistoryLimit defined the number of failed etcdbackups to retain. Failed backups are deleted from the oldest first.
	// Must be non-negative integer.
	// If set to to 0, then failed backups will be deleted immediately.
	// If unset, defaults to 1.
	// Must be a positive integer no greater than 100.
	// This field is optional.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	FailedBackupsHistoryLimit *int32 `json:"failedBackupsHistoryLimit,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="self.type == 'PVC' ? has(self.pvc) : !has(self.pvc)",message="pvc is required when type is PVC, and forbidden otherwise"
// +kubebuilder:validation:XValidation:rule="self.type == 'Local' ? true : !has(self.local)",message="local is optional when type is Local, and forbidden otherwise"
// +union
type EtcdBackupPolicyStorage struct {
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
	PVC EtcdBackupPolicyStoragePvc `json:"pvc,omitzero"`

	// local specifies a host path directory on the control plane node where the etcd backup file will be saved.
	// This field is optional when storage type is "Local", and forbidden otherwise.
	// +optional
	// +unionMember
	Local EtcdBackupPolicyStorageLocal `json:"local,omitzero"`
}

// +kubebuilder:validation:MinProperties=1
type EtcdBackupPolicyStoragePvc struct {
	EtcdBackupStoragePvc `json:",inline"`
}

// +kubebuilder:validation:MinProperties=1
type EtcdBackupPolicyStorageLocal struct {
	EtcdBackupStorageLocal `json:",inline"`

	// distribution controls whether or not scheduled local backups are only taken on a single node or fan out to all control plane nodes.
	// Allowed values are SingleNode and AllNodes
	// When set to SingleNode, a single EtcdBackup will be created at each schedule execution on one of the selected control plane nodes. If multiple nodes are matched by nodeSelector, then backups round robin the nodes in lexicographical order.
	// When set to AllNodes, EtcdBackups will be created for all selected control plane nodes at each schedule execution. Retention rules will be applied to EtcdBackups grouped by Node (e.g. MaxQuantity 2 results in 2 backups per node, MaxTotalSizeGiB 5 results in 5GiB of backups per node).
	// If omitted, defaults to SingleNode.
	// This field is optional.
	// +kubebuilder:validation:Enum:=SingleNode;AllNodes
	// +optional
	Distribution EtcdBackupPolicyLocalDistributionType `json:"distribution,omitempty"`
}

// EtcdBackupPolicyLocalDistributionType is an enum of the supported Local storage distribution types
type EtcdBackupPolicyLocalDistributionType string

const (
	// EtcdBackupPolicyLocalDistributionTypeSingleNode creates a single EtcdBackup at each schedule execution on one of the selected control plane nodes. If multiple nodes are matched by nodeSelector, then backups round robin the nodes in lexicographical order.
	EtcdBackupPolicyLocalDistributionTypeSingleNode EtcdBackupPolicyLocalDistributionType = "SingleNode"
	// EtcdBackupPolicyLocalDistributionTypeAllNodes creates EtcdBackups for all selected control plane nodes at each schedule execution. Retention rules will be applied to EtcdBackups grouped by Node (e.g. MaxQuantity 2 results in 2 backups per node, MaxTotalSizeGiB 5 results in 5GiB of backups per node).
	EtcdBackupPolicyLocalDistributionTypeAllNodes EtcdBackupPolicyLocalDistributionType = "AllNodes"
)

// +union
// +kubebuilder:validation:XValidation:rule="(self.type == 'Quantity') ? has(self.quantity) : !has(self.quantity)",message="quantity is required when type is Quantity, and forbidden otherwise"
// +kubebuilder:validation:XValidation:rule="(self.type == 'Size') ? has(self.size) : !has(self.size)",message="size is required when type is Size, and forbidden otherwise"
type EtcdBackupPolicyRetentionRule struct {
	// type defined which rule field is set
	// Allowed values are Quantity and Size.
	// When set to Quantity, this rule deletes completed EtcdBackups based on the total count.
	// When set to Size, this rule deletes completed EtcdBackups based on the total aggregate size of backup files.
	// This field is required.
	// +unionDiscriminator
	// +kubebuilder:validation:Enum:=Quantity;Size
	// +required
	Type EtcdBackupPolicyRetentionRuleType `json:"type,omitempty"`

	// quantity enforces the deletion of EtcdBackups belonging to the EtcdBackupPolicy based on total count.
	// This field is required when the rule type is "Quantity", and forbidden otherwise.
	// +optional
	Quantity EtcdBackupPolicyRetentionQuantity `json:"quantity,omitzero"`

	// size enforces the deletion of EtcdBackups belonging to the EtcdBackupPolicy based on total backup file size.
	// At least one backup will always be retained, even if it exceeds the size limit.
	// This field is required when the rule type is "Size", and forbidden otherwise.
	// +optional
	Size EtcdBackupPolicyRetentionSize `json:"size,omitzero"`
}

type EtcdBackupPolicyRetentionRuleType string

const (
	// EtcdBackupPolicyRetentionRuleTypeQuantity deletes completed EtcdBackups based on the total count.
	EtcdBackupPolicyRetentionRuleTypeQuantity EtcdBackupPolicyRetentionRuleType = "Quantity"
	// EtcdBackupPolicyRetentionRuleTypeSize deletes completed EtcdBackups based on the total aggregate size of backup files.
	EtcdBackupPolicyRetentionRuleTypeSize EtcdBackupPolicyRetentionRuleType = "Size"
)

type EtcdBackupPolicyRetentionQuantity struct {
	// maxQuantity enforces the deletion of completed backups that exceed the total count.
	// The total count of completed backups may temporarily exceed the limit when new backups are completed. Older backups are not deleted until a new one has succeeded.
	// Must be between 1 and 1000.
	// This field is required.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	// +required
	MaxQuantity int32 `json:"maxQuantity,omitempty"`
}

type EtcdBackupPolicyRetentionSize struct {
	// maxTotalSizeGiB enforces the deletion of completed backups by the total size of backups on the storage backend (in GiB).
	// This is a soft threshold. The total size of backups may temporarily exceed the limit when new backups are completed. Older backups are not deleted until a new one has succeeded.
	// If the newest backup exceeds the limit on its own, it will be retained with all older backups being pruned to ensure at least one valid backup exists.
	// Must be between 1 and 10240 (10 TiB).
	// This field is required.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10240
	// +required
	MaxTotalSizeGiB int32 `json:"maxTotalSizeGiB,omitempty"`
}

// +kubebuilder:validation:MinProperties=1
type EtcdBackupPolicyStatus struct {
	// conditions provide details on the status of the etcd backup policy.
	// Supported conditions include Scheduled.
	// If Scheduled is true, scheduling the policy succeeded.
	// If Scheduled is false, an error occurred while attempting to schedule the policy. The condition reason and message will provide addition details.
	//
	// When specified, must contain between 1 and 10 entries, with no more than 1 of each type.
	// This field is optional.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=10
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type EtcdBackupPolicyConditionType string

const (
	// BackupPolicyScheduled is True if the EtcdBackupPolicy was successfully scheduled, or False if there is an error.
	// Typically will remain True once set, but will be set False if schedule or timeZone are changed to invalid values, or CronJob is manually deleted and fails to recreate.
	BackupPolicyScheduled EtcdBackupPolicyConditionType = "Scheduled"
)

type EtcdBackupPolicyConditionReason string

const (
	// BackupPolicyInvalidSchedule is set when parsing the schedule or timeZone fails.
	BackupPolicyInvalidSchedule EtcdBackupPolicyConditionReason = "InvalidSchedule"
	// BackupPolicySchedulingFailed is set when there was an error applying a valid schedule.
	BackupPolicySchedulingFailed EtcdBackupPolicyConditionReason = "SchedulingFailed"
	// BackupPolicySchedulingSucceeded is set when the policy has been successfully scheduled.
	BackupPolicySchedulingSucceeded EtcdBackupPolicyConditionReason = "SchedulingSucceeded"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EtcdBackupPolicyList is a collection of items
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
type EtcdBackupPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []EtcdBackupPolicy `json:"items"`
}
