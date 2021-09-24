/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package csiclient

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	csiext "github.com/dell/dell-csi-extensions/volumeGroupSnapshot"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
)

type VolumeGroupSnapshot interface {
	CreateVolumeGroupSnapshot(string, []string, map[string]string) (*csiext.CreateVolumeGroupSnapshotResponse, error)
	ProbeController() (string, error)
	ProbeDriver() (string, error)
}

type volumeGroupSnapshot struct {
	conn    *grpc.ClientConn
	log     logr.Logger
	timeout time.Duration
}

func New(conn *grpc.ClientConn, log logr.Logger, timeout time.Duration) *volumeGroupSnapshot {
	return &volumeGroupSnapshot{
		conn:    conn,
		log:     log,
		timeout: timeout,
	}
}

func (v *volumeGroupSnapshot) CreateVolumeGroupSnapshot(vgName string, volIds []string,
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

func (v *volumeGroupSnapshot) ProbeController() (string, error) {
	v.log.V(1).Info("Probing controller")
	ctx, cancel := context.WithTimeout(context.Background(), v.timeout)
	defer cancel()

	client := csiext.NewVolumeGroupSnapshotClient(v.conn)

	response, err := client.ProbeController(ctx, &csiext.ProbeControllerRequest{})
	if err != nil {
		return "", err
	}
	driverName := response.GetName()
	return driverName, nil
}

func (v *volumeGroupSnapshot) ProbeDriver() (string, error) {
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
