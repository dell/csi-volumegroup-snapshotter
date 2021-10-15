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
	csi_ext "github.com/dell/dell-csi-extensions/volumeGroupSnapshot"
	"google.golang.org/grpc"
)

// mock VolumeGroupSnapshotServer
type MockVolumeGroupSnapshotServer struct{}

var MockServer *grpc.Server

func (vgs *MockVolumeGroupSnapshotServer) ProbeController(ctx context.Context, in *csi_ext.ProbeControllerRequest) (*csi_ext.ProbeControllerResponse, error) {
	out := &csi_ext.ProbeControllerResponse{}
	err := FindStub("VolumeGroupSnapshot", "ProbeController", in, out)
	return out, err
}

func (vgs *MockVolumeGroupSnapshotServer) CreateVolumeGroupSnapshot(ctx context.Context, in *csi_ext.CreateVolumeGroupSnapshotRequest) (*csi_ext.CreateVolumeGroupSnapshotResponse, error) {
	out := &csi_ext.CreateVolumeGroupSnapshotResponse{}
	err := FindStub("VolumeGroupSnapshot", "CreateVolumeGroupSnapshot", in, out)
	return out, err
}

func (vgs *MockVolumeGroupSnapshotServer) DeleteVolumeGroupSnapshot(ctx context.Context, in *csi_ext.DeleteVolumeGroupSnapshotRequest) (*csi_ext.DeleteVolumeGroupSnapshotResponse, error) {
	out := &csi_ext.DeleteVolumeGroupSnapshotResponse{}
	err := FindStub("VolumeGroupSnapshot", "DeleteVolumeGroupSnapshot", in, out)
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

// starts a mock server
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

	csi_ext.RegisterVolumeGroupSnapshotServer(MockServer, &MockVolumeGroupSnapshotServer{})

	fmt.Printf("Serving gRPC on %s\n", csiAddress)
	errChan := make(chan error)

	// run blocking call in a separate goroutine, report errors via channel
	go func() {
		if err := MockServer.Serve(lis); err != nil {
			errChan <- err
		}
	}()
}

// stop mock server gracefully
func StopMockServer() {
	MockServer.GracefulStop()
	fmt.Printf("Server stopped gracefully")
}
