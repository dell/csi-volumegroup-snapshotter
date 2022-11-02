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

package connection

import (
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"k8s.io/klog/v2"
)

const connectionLoggingInterval = 10 * time.Second

//Connect to address
func Connect(address string) (*grpc.ClientConn, error) {
	dialOptions := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoff.DefaultConfig,
			MinConnectTimeout: 10 * time.Second,
		}),
		grpc.WithBlock(),
	}
	unixPrefix := "unix://"
	if strings.HasPrefix(address, "/") {
		address = unixPrefix + address
	}

	klog.Infof("Connecting to %s", address)
	var conn *grpc.ClientConn
	var err error
	ready := make(chan bool)
	go func() {
		conn, err = grpc.Dial(address, dialOptions...)
		close(ready)
	}()

	ticker := time.NewTicker(connectionLoggingInterval)
	defer ticker.Stop()

	// Wait until dial finishes
	for {
		select {
		case <-ticker.C:
			klog.Warningf("Still connecting to %s", address)
		case <-ready:
			return conn, err
		}
	}
}
