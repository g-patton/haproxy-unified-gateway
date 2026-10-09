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
	"testing"

	"github.com/haproxytech/haproxy-unified-gateway/test/integration/base"

	"github.com/stretchr/testify/suite"
)

// NoBackendRefsSuite runs the backendless-rules fixtures twice: with the
// controller's default responses and with Gateway API compatibility mode.
// gwapiCompat selects the mode for the whole suite run.
type NoBackendRefsSuite struct {
	base.BaseSuite
	gwapiCompat bool
	echoServer  *echoServer
}

func TestNoBackendRefsDefaultSuite(t *testing.T) {
	suite.Run(t, new(NoBackendRefsSuite))
}

func TestNoBackendRefsCompatSuite(t *testing.T) {
	suite.Run(t, &NoBackendRefsSuite{gwapiCompat: true})
}

func (s *NoBackendRefsSuite) SetupSuite() {
	s.BaseSuite.SetupSuite("", 0, func(test *base.IntTest) {
		test.GWAPICompat = s.gwapiCompat
	})
	s.echoServer = startEchoServer(s.T())
}

func (s *NoBackendRefsSuite) TearDownSuite() {
	s.echoServer.stop()
	s.BaseSuite.TearDownSuite()
}
