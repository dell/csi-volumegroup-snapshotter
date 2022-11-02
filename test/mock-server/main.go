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

package main

import (
	"flag"
)

var (
	csiAddress string
	stubs      string
	apiPort    string
)

func init() {
	flag.StringVar(&csiAddress, "csi-address", "/var/run/csi.sock", "Address of the grpc server")
	flag.StringVar(&stubs, "stubs", "./stubs", "Location of the stubs directory")
	flag.StringVar(&apiPort, "apiPorts", "4771", "API port")
	flag.Parse()
}

// implement start mock grpc server
func main() {
	// TODO
}
