package testvg

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"

	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
<<<<<<< HEAD
	"github.com/dell/csi-vxflexos/v2/provider"
	"github.com/dell/csi-vxflexos/v2/service"
=======
	"github.com/dell/csi-vxflexos/provider"
	"github.com/dell/csi-vxflexos/service"
>>>>>>> a6291e3... add
	"github.com/dell/gocsi/utils"
	"google.golang.org/grpc"
)

const (
	datafile   string = "/tmp/datafile"
	datadir    string = "/tmp/datadir"
	configFile string = "./config.json"
)

var grpcClient *grpc.ClientConn

var sock = "/tmp/unix_sock"

// ArrayConnectionData contains data required to connect to array
type ArrayConnectionData struct {
	SystemID  string `json:"systemID"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Endpoint  string `json:"endpoint"`
	Insecure  bool   `json:"insecure,omitempty"`
	IsDefault bool   `json:"isDefault,omitempty"`
}

// read config.json and confirm its good
func init() {
	// load array config and give proper errors if not ok
	if _, err := os.Stat(configFile); err == nil {
		if _, err := ioutil.ReadFile(configFile); err != nil {
			err = fmt.Errorf("DEBUG integration pre requisites missing %s with multi array configuration file ", configFile)
			panic(err)
		}
		f, _ := os.Open(configFile)
		r := bufio.NewReader(f)
		mdms := make([]string, 0)
		_, isPrefix, err := r.ReadLine()
		for err == nil && !isPrefix {
			line, _, _ := r.ReadLine()
			if strings.Contains(string(line), "127.0.0.1") {
				err := fmt.Errorf("integration test pre-requisite powerflex array endpoint %s is not ok, setup ../../config.json", string(line))
				panic(err)
			}
			if strings.Contains(string(line), "mdm") {
				mdms = append(mdms, string(line))
			}
			if len(line) < 1 {
				break
			}
		}
		if len(mdms) < 1 {
			err := fmt.Errorf("Integration Test pre-requisite config file ../../config.json  must have mdm key set with working ip %#v", mdms)
			panic(err)
		}
	} else if os.IsNotExist(err) {
		err := fmt.Errorf("Integration Test pre-requisite needs  a valid config.json located here :  %s\"", err)
		panic(err)
	}
}

<<<<<<< HEAD
<<<<<<< HEAD
//TestDummy not used
=======
>>>>>>> a6291e3... add
=======
//TestDummy not used
>>>>>>> 4fcacba... fix lint erros
func TestDummy(t *testing.T) {

}

<<<<<<< HEAD
<<<<<<< HEAD
//TestMain similar to driver integration test
=======
// similar to driver integration test
>>>>>>> a6291e3... add
=======
//TestMain similar to driver integration test
>>>>>>> 4fcacba... fix lint erros
// use godog feature to run init and cleanup steps before and after all scenarios
func TestMain(m *testing.M) {
	var stop func()
	ctx := context.Background()
	_ = os.Remove(sock)
	fmt.Printf("calling startServer\n")
	grpcClient, stop = startServer(ctx)

	fmt.Print("waiting for client ...\n")
	time.Sleep(5 * time.Second)

	var opts = godog.Options{
		Format: "pretty", // can define default values
		Paths:  []string{"features"},
		// Tags:   "wip",
	}
	exitVal := godog.TestSuite{
		Name:                 "godog",
		TestSuiteInitializer: CleanupTestSuite,
		ScenarioInitializer:  VGFeatureContext,
		Options:              &opts,
	}.Run()

	if st := m.Run(); st > exitVal {
		exitVal = st
	}

	stop()
	os.Exit(exitVal)
}

func startServer(ctx context.Context) (*grpc.ClientConn, func()) {
	// Create a new SP instance and serve it with a piped connection.
	sp := provider.New()
	service.ArrayConfigFile = configFile
	lis, err := utils.GetCSIEndpointListener()
	if err != nil {
		fmt.Printf("couldn't open listener: %s\n", err.Error())
		return nil, nil
	}
	fmt.Printf("lis: %v\n", lis)
	go func() {
		fmt.Printf("starting server\n")
		if err := sp.Serve(ctx, lis); err != nil {
			fmt.Printf("serve error: %s\n", err.Error())
			fmt.Printf("http: Server closed")
			_, addr, _ := utils.GetCSIEndpoint()
			_ = os.Remove(addr)
			os.Exit(1)
		}
	}()
	network, addr, err := utils.GetCSIEndpoint()
	if err != nil {
		return nil, nil
	}
	fmt.Printf("network %v addr %v\n", network, addr)
	clientOpts := []grpc.DialOption{
		grpc.WithInsecure(),
	}

	// Create a client for the piped connection.
	fmt.Printf("calling gprc.DialContext, ctx %v, addr %s, clientOpts %v\n", ctx, addr, clientOpts)
	client, err := grpc.DialContext(ctx, "unix:"+addr, clientOpts...)
	if err != nil {
		fmt.Printf("DialContext returned error: %s", err.Error())
	}
	fmt.Printf("grpc.DialContext returned ok\n")

	return client, func() {
		_ = client.Close()
		sp.GracefulStop(ctx)
	}
}

// there is no way to call service.go methods from here
// hence copy same method over there , this is used to get all arrays and pick different
// systemID to test with see  method iSetAnotherSystemID
func getArrayConfig() (map[string]*ArrayConnectionData, error) {
	arrays := make(map[string]*ArrayConnectionData)

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf(fmt.Sprintf("File %s does not exist", configFile))
	}

	config, err := ioutil.ReadFile(filepath.Clean(configFile))
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("File %s errors: %v", configFile, err))
	}

	if string(config) != "" {
		jsonCreds := make([]ArrayConnectionData, 0)
		err := json.Unmarshal(config, &jsonCreds)
		if err != nil {
			return nil, fmt.Errorf(fmt.Sprintf("Unable to parse the credentials: %v", err))
		}

		if len(jsonCreds) == 0 {
			return nil, fmt.Errorf("no arrays are provided in configFile %s", configFile)
		}

		noOfDefaultArray := 0
		for i, c := range jsonCreds {
			systemID := c.SystemID
			if _, ok := arrays[systemID]; ok {
				return nil, fmt.Errorf(fmt.Sprintf("duplicate system ID %s found at index %d", systemID, i))
			}
			if systemID == "" {
				return nil, fmt.Errorf(fmt.Sprintf("invalid value for system name at index %d", i))
			}
			if c.Username == "" {
				return nil, fmt.Errorf(fmt.Sprintf("invalid value for Username at index %d", i))
			}
			if c.Password == "" {
				return nil, fmt.Errorf(fmt.Sprintf("invalid value for Password at index %d", i))
			}
			if c.Endpoint == "" {
				return nil, fmt.Errorf(fmt.Sprintf("invalid value for Endpoint at index %d", i))
			}

			fields := map[string]interface{}{
				"endpoint":  c.Endpoint,
				"user":      c.Username,
				"password":  "********",
				"insecure":  c.Insecure,
				"isDefault": c.IsDefault,
				"systemID":  c.SystemID,
			}

			fmt.Printf("array found  %s %#v\n", c.SystemID, fields)

			if c.IsDefault {
				noOfDefaultArray++
			}

			if noOfDefaultArray > 1 {
				return nil, fmt.Errorf("'isDefault' parameter presents more than once in storage array list")
			}

			// copy in the arrayConnectionData to arrays
			copy := ArrayConnectionData{}
			copy = c
			arrays[c.SystemID] = &copy
		}
	} else {
		return nil, fmt.Errorf("arrays details are not provided in configFile %s", configFile)
	}
	return arrays, nil
}
