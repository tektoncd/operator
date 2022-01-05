package triggers

const (
	// GroupName is the Kubernetes resource group name for Tekton types.
	GroupName = "triggers.tekton.dev"

	// EventListenerLabelKey is used as the label identifier for an EventListener.
	EventListenerLabelKey = "/eventlistener"

	// EventIDLabelKey is used as the label identifier for an EventListener event.
	EventIDLabelKey = "/triggers-eventid"

	// TriggerLabelKey is used as the label identifier for a Trigger
	TriggerLabelKey = "/trigger"

	// TriggerGroupLabelKey is used as a label identifier for a TriggerGroup
	TriggerGroupLabelKey = "/triggergroup"
)
