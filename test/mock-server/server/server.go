// Copyright © 2021 - 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"github.com/dell/csi-volumegroup-snapshotter/test/mock-server/stub"
	csiext "github.com/dell/dell-csi-extensions/common"
	vgsext "github.com/dell/dell-csi-extensions/volumeGroupSnapshot"
	"google.golang.org/grpc"
)

// MockVolumeGroupSnapshotServer mock VolumeGroupSnapshotServer
type MockVolumeGroupSnapshotServer struct{}

// MockServer grpc server handle
var MockServer *grpc.Server

// ProbeController grpc call to get driver information
func (vgs *MockVolumeGroupSnapshotServer) ProbeController(_ context.Context, in *csiext.ProbeControllerRequest) (*csiext.ProbeControllerResponse, error) {
	out := &csiext.ProbeControllerResponse{}
	err := FindStub("VolumeGroupSnapshot", "ProbeController", in, out)
	return out, err
}

// CreateVolumeGroupSnapshot creete vgs
func (vgs *MockVolumeGroupSnapshotServer) CreateVolumeGroupSnapshot(_ context.Context, in *vgsext.CreateVolumeGroupSnapshotRequest) (*vgsext.CreateVolumeGroupSnapshotResponse, error) {
	out := &vgsext.CreateVolumeGroupSnapshotResponse{}
	err := FindStub("VolumeGroupSnapshot", "CreateVolumeGroupSnapshot", in, out)
	return out, err
}

type payload struct {
	Service string      `json:"service"`
	Method  string      `json:"method"`
	Data    interface{} `json:"data"`
}

type response struct {
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

// FindStub post find request and returns the unmarshalled response
func FindStub(service, method string, in, out interface{}) error {
	url := "http://localhost:4771/find"
	pyl := payload{
		Service: service,
		Method:  method,
		Data:    in,
	}
	byt, err := json.Marshal(pyl)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(byt)
	resp, err := http.DefaultClient.Post(url, "application/json", reader)
	if err != nil {
		return fmt.Errorf("error request to stub server %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf(string(body))
	}

	respRPC := new(response)
	err = json.NewDecoder(resp.Body).Decode(respRPC)
	if err != nil {
		return fmt.Errorf("decoding json response error %v", err)
	}

	if respRPC.Error != "" {
		return fmt.Errorf("response RPC error %v", respRPC.Error)
	}

	data, _ := json.Marshal(respRPC.Data)
	return json.Unmarshal(data, out)
}

// RunServer starts a mock server
func RunServer(stubsPath string) {
	fmt.Print("RUNNING MOCK SERVER")
	const (
		csiAddress = "localhost:4772"
		// relate path from stub.go to stubs dir
		defaultStubsPath = "../stubs"
		apiPort          = "4771"
	)

	if len(stubsPath) == 0 {
		stubsPath = defaultStubsPath
	}

	// run admin stub server
	stub.RunStubServer(stub.Options{
		StubPath: stubsPath,
		Port:     apiPort,
		BindAddr: "0.0.0.0",
	})

	var protocol string
	if strings.Contains(csiAddress, ":") {
		protocol = "tcp"
	} else {
		protocol = "unix"
	}

	lis, err := net.Listen(protocol, csiAddress)
	if err != nil {
		fmt.Printf("failed to listen on address [%s]: %s", csiAddress, err.Error())
		return
	}

	MockServer = grpc.NewServer()

	vgsext.RegisterVolumeGroupSnapshotServer(MockServer, &MockVolumeGroupSnapshotServer{})

	fmt.Printf("Serving gRPC on %s\n", csiAddress)
	errChan := make(chan error)

	// run blocking call in a separate goroutine, report errors via channel
	go func() {
		if err := MockServer.Serve(lis); err != nil {
			errChan <- err
		}
	}()
}

// StopMockServer stop mock server gracefully
func StopMockServer() {
	MockServer.GracefulStop()
	fmt.Printf("Server stopped gracefully")
}
