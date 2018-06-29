package custom_if_prefix

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	mathutils "github.com/projectcalico/libnetwork-plugin/utils/math"
	. "github.com/projectcalico/libnetwork-plugin/utils/test"
)

var _ = Describe("Running plugin with custom ENV", func() {
	Describe("docker run", func() {
		It("creates a container on a network with correct IFPREFIX", func() {
			// Run the plugin with custom IFPREFIX
			RunPlugin("-e CALICO_LIBNETWORK_IFPREFIX=test")

			pool := "test"
			subnet := "192.169.0.0/16"
			// Since running the plugin starts etcd, the pool needs to be created after.
			CreatePool(pool, subnet)

			name := fmt.Sprintf("run%d", rand.Uint32())
			nid := DockerString(fmt.Sprintf("docker network create --driver calico --ipam-driver calico-ipam --subnet %s %s ", subnet, pool))
			UpdatePool(pool, subnet, nid)

			// Create a container that will just sit in the background
			DockerString(fmt.Sprintf("docker run --net %s -tid --name %s %s", pool, name, os.Getenv("BUSYBOX_IMAGE")))

			// Gather information for assertions
			dockerEndpoint := GetDockerEndpoint(name, pool)
			ip := dockerEndpoint.IPAddress
			mac := dockerEndpoint.MacAddress
			endpointID := dockerEndpoint.EndpointID
			nicName := "cali" + endpointID[:mathutils.MinInt(11, len(endpointID))]

			// Check that the endpoint is created in etcd
			key := fmt.Sprintf("/calico/resources/v3/projectcalico.org/workloadendpoints/%s/%s-libnetwork-libnetwork-%s", "libnetwork", pool, endpointID)
			endpointJSON := GetEtcd(key)
			wep := map[string]interface{}{}
			json.Unmarshal(endpointJSON, &wep)
			spec := wep["spec"].(map[string]interface{})
			Expect(spec["interfaceName"].(string)).Should(Equal(nicName))

			//// Check profile
			profileJSON := GetEtcd(fmt.Sprintf("/calico/resources/v3/projectcalico.org/profiles/%s", pool))
			profile := map[string]interface{}{}
			json.Unmarshal(profileJSON, &profile)
			meta := profile["metadata"].(map[string]interface{})
			Expect(meta["name"].(string)).Should(Equal(pool))

			// Check the interface exists on the Host - it has an autoassigned
			// mac and ip, so don't check anything!
			DockerString(fmt.Sprintf("ip addr show %s", nicName))

			// Make sure the interface in the container exists and has the  assigned ip and mac
			containerNICString := DockerString(fmt.Sprintf("docker exec -i %s ip addr", name))
			Expect(containerNICString).Should(ContainSubstring(ip))
			Expect(containerNICString).Should(ContainSubstring(mac))

			// Make sure the container has the routes we expect
			routes := DockerString(fmt.Sprintf("docker exec -i %s ip route", name))
			Expect(routes).Should(Equal("default via 169.254.1.1 dev test0 \n169.254.1.1 dev test0 scope link"))

			// Delete container
			DockerString(fmt.Sprintf("docker rm -f %s", name))
		})
	})
})
