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
