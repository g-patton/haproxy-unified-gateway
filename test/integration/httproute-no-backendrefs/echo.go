// Copyright 2026 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package httproutenobackendrefs

import (
	"errors"
	"net"
	"net/http"
	"testing"
)

const echoResponseBody = "echo-backend"

// echoServer is a local HTTP backend wired into the cluster through a ready
// EndpointSlice, so catchall traffic is proxied for real instead of being
// asserted through map goldens.
type echoServer struct {
	server  *http.Server
	address string
	port    int
}

func startEchoServer(t *testing.T) *echoServer {
	address, err := localNonLoopbackAddress(t)
	if err != nil {
		t.Fatalf("could not find a non-loopback local address: %v", err)
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(address, "0"))
	if err != nil {
		t.Fatalf("could not start echo server: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(echoResponseBody))
	})
	srv := &http.Server{Handler: mux}
	go func() {
		_ = srv.Serve(listener)
	}()
	return &echoServer{
		server:  srv,
		address: address,
		port:    listener.Addr().(*net.TCPAddr).Port,
	}
}

func (e *echoServer) stop() {
	_ = e.server.Close()
}

// localNonLoopbackAddress picks a routable local address: envtest rejects
// loopback endpoint addresses, so the echo server must listen elsewhere.
func localNonLoopbackAddress(t *testing.T) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			t.Logf("skipping interface %s: %v", iface.Name, err)
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ip := ipnet.IP.To4(); ip != nil && !ip.IsLoopback() {
					return ip.String(), nil
				}
			}
		}
	}
	return "", errors.New("no non-loopback IPv4 interface address found")
}
