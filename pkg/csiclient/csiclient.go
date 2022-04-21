package csiclient

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonext "github.com/dell/dell-csi-extensions/common"
	csiext "github.com/dell/dell-csi-extensions/volumeGroupSnapshot"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
)

//VolumeGroupSnapshot grpc calls to driver
type VolumeGroupSnapshot interface {
	CreateVolumeGroupSnapshot(string, []string, map[string]string) (*csiext.CreateVolumeGroupSnapshotResponse, error)
	ProbeController() (string, error)
	ProbeDriver() (string, error)
}

//VolumeGroupSnapshotClient vg controller
type VolumeGroupSnapshotClient struct {
	conn    *grpc.ClientConn
	log     logr.Logger
	timeout time.Duration
}

//New csiclient
func New(conn *grpc.ClientConn, log logr.Logger, timeout time.Duration) *VolumeGroupSnapshotClient {
	return &VolumeGroupSnapshotClient{
		conn:    conn,
		log:     log,
		timeout: timeout,
	}
}

//CreateVolumeGroupSnapshot grpc call to driver
func (v *VolumeGroupSnapshotClient) CreateVolumeGroupSnapshot(vgName string, volIds []string,
	params map[string]string) (*csiext.CreateVolumeGroupSnapshotResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), v.timeout)
	defer cancel()

	client := csiext.NewVolumeGroupSnapshotClient(v.conn)

	req := &csiext.CreateVolumeGroupSnapshotRequest{
		SourceVolumeIDs: volIds,
		Name:            vgName,
		Parameters:      params,
	}

	return client.CreateVolumeGroupSnapshot(ctx, req)
}

//ProbeController grpc call to driver
func (v *VolumeGroupSnapshotClient) ProbeController() (string, error) {
	v.log.V(1).Info("Probing controller")
	ctx, cancel := context.WithTimeout(context.Background(), v.timeout)
	defer cancel()

	client := csiext.NewVolumeGroupSnapshotClient(v.conn)

	response, err := client.ProbeController(ctx, &commonext.ProbeControllerRequest{})
	if err != nil {
		return "", err
	}
	driverName := response.GetName()
	return driverName, nil
}

//ProbeDriver wrapper for grpc call
func (v *VolumeGroupSnapshotClient) ProbeDriver() (string, error) {
	for {
		v.log.V(2).Info("Probing driver for readiness")
		driverName, err := v.ProbeController()
		if err != nil {
			st, ok := status.FromError(err)
			if !ok {
				// Not a grpc error; probe failed before grpc method was called
				return "", err
			}
			if st.Code() != codes.DeadlineExceeded {
				return "", err
			}
			v.log.V(1).Info("CSI driver probe timed out")
		} else {
			return driverName, nil
		}
		time.Sleep(time.Second)
	}
}
