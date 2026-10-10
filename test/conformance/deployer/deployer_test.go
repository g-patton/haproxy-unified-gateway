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

package deployer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildDeploymentFieldRefsDeclareAPIVersion pins the version on every
// field reference of the generated container environment: the API server fills
// it in when storing the Deployment, so without it the desired and stored
// specs never compare equal and every reconcile rewrites the Deployment,
// restarting the controller under the conformance tests.
func TestBuildDeploymentFieldRefsDeclareAPIVersion(t *testing.T) {
	r := &GatewayReconciler{
		Config: Config{
			DeployerNs:      "haproxy-unified-gateway",
			ControllerImage: "haproxytech/haproxy-unified-gateway:latest",
			HugConfCRD:      "haproxy-unified-gateway/hugconf",
		},
	}

	deploy := r.buildDeployment(
		"hug-0123456789abcdef",
		"gateway-conformance-infra",
		"gw-test",
		"gateway-conformance-infra/gw-test",
		"app.kubernetes.io/name:hug_gateway-conformance-infra_gw-test",
	)

	fieldRefs := 0
	for _, env := range deploy.Spec.Template.Spec.Containers[0].Env {
		if env.ValueFrom == nil || env.ValueFrom.FieldRef == nil {
			continue
		}
		fieldRefs++
		require.Equal(t, "v1", env.ValueFrom.FieldRef.APIVersion,
			"field ref %s must declare its api version", env.Name)
	}
	require.NotZero(t, fieldRefs, "generated controller containers must use field references")
}
