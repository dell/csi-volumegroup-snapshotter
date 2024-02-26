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

package stub

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi"
)

// Options is used for RunStubServer
type Options struct {
	Port     string
	BindAddr string
	StubPath string
}

// Stub is used for json stub file marshall
type Stub struct {
	Service string `json:"service"`
	Method  string `json:"method"`
	Input   Input  `json:"input"`
	Output  Output `json:"output"`
}

// Input is part of Stub struct
type Input struct {
	Equals   map[string]interface{} `json:"equals"`
	Contains map[string]interface{} `json:"contains"`
	Matches  map[string]interface{} `json:"matches"`
}

// Output is part of Stub struct
type Output struct {
	Data  map[string]interface{} `json:"data"`
	Error string                 `json:"error"`
}

const (
	// DefaultAddress for test
	DefaultAddress = "localhost"
	// DefaultPort for test
	DefaultPort = "4771"
	// TimeOut for test
	TimeOut = 10 * time.Minute
)

// RunStubServer test server
func RunStubServer(opt Options) {
	if opt.Port == "" {
		opt.Port = DefaultPort
	}
	if opt.BindAddr == "" {
		opt.BindAddr = DefaultAddress
	}
	addr := opt.BindAddr + ":" + opt.Port
	r := chi.NewRouter()
	r.Post("/add", handleAddStub)
	r.Get("/", handleListStub)
	r.Post("/find", handleFindStub)
	r.Get("/clear", handleClearStub)

	if opt.StubPath != "" {
		readStubFromDir(opt.StubPath)
	}

	fmt.Println("Serving stub admin on http://" + addr)
	go func() {
		server := &http.Server{
			Addr:              addr,
			ReadHeaderTimeout: TimeOut,
			Handler:           r,
		}
		err := server.ListenAndServe()
		log.Println(err)
	}()
}

func responseError(err error, w http.ResponseWriter) {
	w.WriteHeader(500)
	_, e := w.Write([]byte(err.Error()))
	if e != nil {
		log.Printf("responseError write error: %v", e)
	}
}

func handleAddStub(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		responseError(err, w)
		return
	}

	stub := new(Stub)
	err = json.Unmarshal(body, stub)
	if err != nil {
		responseError(err, w)
		return
	}

	err = validateStub(stub)
	if err != nil {
		responseError(err, w)
		return
	}

	err = storeStub(stub)
	if err != nil {
		responseError(err, w)
		return
	}

	_, err = w.Write([]byte("Success add stub"))
	if err != nil {
		log.Printf("handleAddStub write error: %v", err)
	}
}

func handleListStub(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(allStub())
	if err != nil {
		log.Printf("handleListStub write error: %v", err)
	}
}

func validateStub(stub *Stub) error {
	if stub.Service == "" {
		return fmt.Errorf("service name can't be empty")
	}

	if stub.Method == "" {
		return fmt.Errorf("method name can't be emtpy")
	}

	// due to golang implementation
	// method name must capital
	stub.Method = strings.Title(stub.Method)

	switch {
	case stub.Input.Contains != nil:
		break
	case stub.Input.Equals != nil:
		break
	case stub.Input.Matches != nil:
		break
	default:
		return fmt.Errorf("input cannot be empty")
	}

	// TODO: validate all input case

	if stub.Output.Error == "" && stub.Output.Data == nil {
		return fmt.Errorf("Output can't be empty")
	}
	return nil
}

type findStubPayload struct {
	Service string                 `json:"service"`
	Method  string                 `json:"method"`
	Data    map[string]interface{} `json:"data"`
}

func handleFindStub(w http.ResponseWriter, r *http.Request) {
	stub := new(findStubPayload)
	err := json.NewDecoder(r.Body).Decode(stub)
	if err != nil {
		responseError(err, w)
		return
	}

	// due to golang implementation
	// method name must capital
	stub.Method = strings.Title(stub.Method)

	output, err := findStub(stub)
	if err != nil {
		log.Println(err)
		responseError(err, w)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(output)
	if err != nil {
		log.Printf("handleFindStub write error: %v", err)
	}
}

func handleClearStub(w http.ResponseWriter, _ *http.Request) {
	clearStorage()
	_, err := w.Write([]byte("OK"))
	if err != nil {
		log.Printf("handleClearStub write error: %v", err)
	}
}
