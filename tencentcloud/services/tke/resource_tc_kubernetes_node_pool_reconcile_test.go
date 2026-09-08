package tke

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	tkesdk "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tke/v20180525"
)

// This is the node_config hydration path called by the real Read hook. Upjet
// can reconstruct state with this block empty without invoking SDK Importer.
func TestNodePoolReadRepairsMissingNodeConfig(t *testing.T) {
	for _, podCount := range []int64{16, 32} {
		d := ResourceTencentCloudKubernetesNodePool().Data(&terraform.InstanceState{
			ID: "cls-example#np-example", Attributes: map[string]string{"node_config.#": "0"},
		})
		cloud := &tkesdk.NodePool{DesiredPodNum: &podCount}
		if err := readNodePoolNodeConfig(d, cloud, false); err != nil {
			t.Fatal(err)
		}
		if got := d.Get("node_config.0.desired_pod_num"); got != int(podCount) {
			t.Fatalf("ordinary Read retained pod count %v, want cloud %d", got, podCount)
		}
	}
}

// An unrelated script/tag update must not write back an autoscaler's last
// observed capacity. The real Update hook calls this predicate before bounds.
func TestNodePoolUnrelatedUpdateDoesNotSetCapacity(t *testing.T) {
	for _, capacity := range []int{0, 3, 10} {
		if nodePoolCapacityWriteFromSDKUpdate(t, capacity, nil) {
			t.Fatalf("script-only update would write capacity %d", capacity)
		}
	}
}

func TestNodePoolExplicitCapacityChangeRetained(t *testing.T) {
	for _, capacity := range []int{0, 2, 8} {
		if !nodePoolCapacityWriteFromSDKUpdate(t, 3, &capacity) {
			t.Fatalf("explicit capacity change 3 -> %d was lost", capacity)
		}
	}
}

// Drive SDK Diff -> Apply to obtain the actual Update ResourceData. d.Set on
// read-only ResourceData does not construct a Terraform update diff.
func nodePoolCapacityWriteFromSDKUpdate(t *testing.T, oldCapacity int, configured *int) bool {
	t.Helper()
	r := ResourceTencentCloudKubernetesNodePool()
	config := func(capacity *int, script string) map[string]interface{} {
		c := map[string]interface{}{
			"cluster_id": "cls-example", "name": "example", "vpc_id": "vpc-example",
			"min_size": 0, "max_size": 100,
			"auto_scaling_config": []interface{}{map[string]interface{}{"instance_type": "M9.2XLARGE64"}},
			"node_config":         []interface{}{map[string]interface{}{"user_data": script, "desired_pod_num": 16}},
		}
		if capacity != nil {
			c["desired_capacity"] = *capacity
		}
		return c
	}
	d := schema.TestResourceDataRaw(t, r.Schema, config(&oldCapacity, "old-script"))
	d.SetId("cls-example#np-example")
	state := d.State()
	diff, err := r.Diff(context.Background(), state, terraform.NewResourceConfigRaw(config(configured, "new-script")), nil)
	if err != nil {
		t.Fatal(err)
	}
	if diff == nil || diff.RequiresNew() {
		t.Fatal("expected an in-place SDK update diff")
	}
	called, write := false, false
	r.Update = func(update *schema.ResourceData, _ interface{}) error {
		called = true
		write = shouldUpdateNodePoolCapacityBeforeBounds(update)
		return nil
	}
	if _, diagnostics := r.Apply(context.Background(), state, diff, nil); diagnostics.HasError() {
		t.Fatalf("SDK Apply: %v", diagnostics)
	}
	if !called {
		t.Fatal("SDK did not invoke Update")
	}
	return write
}

func TestNodePoolNormalReadPreservesPopulatedConfig(t *testing.T) {
	d := ResourceTencentCloudKubernetesNodePool().Data(&terraform.InstanceState{
		ID: "cls-example#np-example", Attributes: map[string]string{
			"node_config.#": "1", "node_config.0.desired_pod_num": "32",
			"node_config.0.user_data": "retained-script",
		},
	})
	cloudPodNum := int64(16)
	if err := readNodePoolNodeConfig(d, &tkesdk.NodePool{DesiredPodNum: &cloudPodNum}, false); err != nil {
		t.Fatal(err)
	}
	if d.Get("node_config.0.desired_pod_num") != 32 || d.Get("node_config.0.user_data") != "retained-script" {
		t.Fatal("ordinary read unexpectedly overwrote populated/write-only configuration")
	}
}

func TestNodePoolReadHydrationPreventsReplacement(t *testing.T) {
	for _, importing := range []bool{false, true} {
		t.Run(fmt.Sprintf("importing=%t", importing), func(t *testing.T) {
			r := ResourceTencentCloudKubernetesNodePool()
			base := map[string]interface{}{
				"cluster_id": "cls-example", "name": "example", "vpc_id": "vpc-example",
				"min_size": 0, "max_size": 100,
				"auto_scaling_config": []interface{}{map[string]interface{}{"instance_type": "M9.2XLARGE64"}},
			}
			d := schema.TestResourceDataRaw(t, r.Schema, base)
			d.SetId("cls-example#np-example")
			podNum := int64(16)
			if err := readNodePoolNodeConfig(d, &tkesdk.NodePool{DesiredPodNum: &podNum}, importing); err != nil {
				t.Fatal(err)
			}
			base["node_config"] = []interface{}{map[string]interface{}{"user_data": "new-script"}}
			diff, err := r.Diff(context.Background(), d.State(), terraform.NewResourceConfigRaw(base), nil)
			if err != nil {
				t.Fatal(err)
			}
			if diff != nil && diff.RequiresNew() {
				t.Fatal("read-repaired existing pool still requires replacement")
			}
		})
	}
}

func TestNodePoolOmittedNodeConfigRetainsCloudDefaults(t *testing.T) {
	r := ResourceTencentCloudKubernetesNodePool()
	base := map[string]interface{}{
		"cluster_id": "cls-example", "name": "example", "vpc_id": "vpc-example",
		"min_size": 0, "max_size": 100,
		"auto_scaling_config": []interface{}{map[string]interface{}{"instance_type": "M9.2XLARGE64"}},
	}
	d := schema.TestResourceDataRaw(t, r.Schema, base)
	d.SetId("cls-example#np-example")
	podNum := int64(16)
	if err := readNodePoolNodeConfig(d, &tkesdk.NodePool{DesiredPodNum: &podNum}, false); err != nil {
		t.Fatal(err)
	}
	diff, err := r.Diff(context.Background(), d.State(), terraform.NewResourceConfigRaw(base), nil)
	if err != nil {
		t.Fatal(err)
	}
	if diff != nil && diff.RequiresNew() {
		t.Fatal("omitted node_config must retain observed defaults, not remove the block and replace the pool")
	}
	if diff != nil {
		for key := range diff.Attributes {
			if strings.HasPrefix(key, "node_config.") {
				t.Fatalf("omitted block would drift at %s", key)
			}
		}
	}
}
