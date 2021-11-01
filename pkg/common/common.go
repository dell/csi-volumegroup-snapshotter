package common

// Constants
const (
	// DellCSIVolumegroup - Name of the sidecar controller manager
	DellCSIVolumegroup = "csi-volumegroup-snapshotter"
	LabelSnapshotGroup = "snapshotGroup"

	EventTypeNormal       = "Normal"
	EventTypeWarning      = "Warning"
	EventReasonUpdated    = "Updated"
	EventStatusPending    = "Pending"
	EventStatusError      = "Error"
	EventStatusIncomplete = "Incomplete"
	EventStatusComplete   = "Complete"
	FinalizerName         = "vgFinalizer"
	ExistingGroupID       = "existingSnapshotGroupID"
)
