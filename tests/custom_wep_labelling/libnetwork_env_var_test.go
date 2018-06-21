package custom_wep_labelling

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	mathutils "github.com/projectcalico/libnetwork-plugin/utils/math"
	. "github.com/projectcalico/libnetwork-plugin/utils/test"
)

var _ = Describe("Running plugin with custom ENV", func() {
	Describe("docker run", func() {
		It("creates a container on a network with WEP labelling enabled", func() {
			RunPlugin("-e CALICO_LIBNETWORK_LABEL_ENDPOINTS=true")

			pool := "test"
			subnet := "192.169.1.0/24"
			// Since running the plugin starts etcd, the pool needs to be created after.
			CreatePool(pool, subnet)

			name := fmt.Sprintf("run%d", rand.Uint32())
			nid := DockerString(fmt.Sprintf("docker network create --driver calico --ipam-driver calico-ipam --subnet %s %s ", subnet, pool))
			UpdatePool(pool, subnet, nid)

			// Create a container that will just sit in the background
			DockerString(fmt.Sprintf("docker run --net %s -tid --label not=expected --label org.projectcalico.label.foo=bar --label org.projectcalico.label.baz=quux --name %s %s", pool, name, os.Getenv("BUSYBOX_IMAGE")))

			// Gather information for assertions
			dockerEndpoint := GetDockerEndpoint(name, pool)
			endpointID := dockerEndpoint.EndpointID
			nicName := "cali" + endpointID[:mathutils.MinInt(11, len(endpointID))]

			// Sleep to allow the plugin to query the started container and update the WEP
			// Alternative: query etcd until we hit jackpot or timeout
			time.Sleep(time.Second)

			// Check that the endpoint is created in etcd
			key := fmt.Sprintf("/calico/resources/v3/projectcalico.org/workloadendpoints/%s/%s-libnetwork-libnetwork-%s", pool, pool, endpointID)
			endpointJSON := GetEtcd(key)
			GinkgoWriter.Write(endpointJSON)
			GinkgoWriter.Write([]byte("\n"))
			wep := map[string]interface{}{}
			json.Unmarshal(endpointJSON, &wep)
			spec := wep["spec"].(map[string]interface{})
			Expect(spec["interfaceName"].(string)).Should(Equal(nicName))

			// Check profile
			profileJSON := GetEtcd(fmt.Sprintf("/calico/resources/v3/projectcalico.org/profiles/%s", pool))
			profile := map[string]interface{}{}
			json.Unmarshal(profileJSON, &profile)
			meta := profile["metadata"].(map[string]interface{})
			Expect(meta["name"].(string)).Should(Equal(pool))

			// Delete container
			DockerString(fmt.Sprintf("docker rm -f %s", name))
		})
	})

})
