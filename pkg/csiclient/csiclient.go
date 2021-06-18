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

	csiext "github.com/dell/dell-csi-extensions/volumeGroupSnapshot"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
)

type VolumeGroupSnapshot interface {
	CreateVolumeGroupSnapshot(string, []string, map[string]string) (*csiext.CreateVolumeGroupSnapshotResponse, error)
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

	//v.log.Info("DEBUG CSI GRPC call Triggered")

	client := csiext.NewVolumeGroupSnapshotClient(v.conn)

	req := &csiext.CreateVolumeGroupSnapshotRequest{
		SourceVolumeIDs: volIds,
		Name:            vgName,
		Parameters:      params,
	}

	res, err := client.CreateVolumeGroupSnapshot(ctx, req)

	//v.log.Info("DEBUG CSI GRPC call response ","res",fmt.Sprintf("%#v",res))

	return res, err
}
