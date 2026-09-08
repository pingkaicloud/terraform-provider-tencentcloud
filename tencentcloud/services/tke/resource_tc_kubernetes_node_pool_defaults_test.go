package tke_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	svctke "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/services/tke"
)

// No cloud calls: exercise the same SDK diff used when reconciling an imported
// node pool, including a node_config block containing only user_data.
func TestKubernetesNodePoolDesiredPodNumDefault(t *testing.T) {
	for _, tc := range []struct {
		name            string
		previous        int
		configured      *int
		wantReplacement bool
	}{
		{name: "omitted retains server default", previous: 16},
		{name: "omitted retains non-default cloud value", previous: 32},
		{name: "same explicit value", previous: 16, configured: intPointer(16)},
		{name: "explicit change remains immutable", previous: 16, configured: intPointer(32), wantReplacement: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := svctke.ResourceTencentCloudKubernetesNodePool()
			config := func(podNum *int) map[string]interface{} {
				node := map[string]interface{}{"user_data": "ZWNobyB0ZXN0"}
				if podNum != nil {
					node["desired_pod_num"] = *podNum
				}
				return map[string]interface{}{
					"cluster_id": "cls-example", "name": "example", "vpc_id": "vpc-example",
					"min_size": 0, "max_size": 100, "subnet_ids": []interface{}{"subnet-example"},
					"auto_scaling_config": []interface{}{map[string]interface{}{"instance_type": "M9.2XLARGE64"}},
					"node_config":         []interface{}{node},
				}
			}
			data := schema.TestResourceDataRaw(t, r.Schema, config(&tc.previous))
			data.SetId("cls-example#np-example")
			diff, err := r.Diff(context.Background(), data.State(), terraform.NewResourceConfigRaw(config(tc.configured)), nil)
			if err != nil {
				t.Fatal(err)
			}
			requiresNew := diff != nil && diff.RequiresNew()
			var podNumberDiff *terraform.ResourceAttrDiff
			if diff != nil {
				podNumberDiff = diff.Attributes["node_config.0.desired_pod_num"]
			}
			if requiresNew != tc.wantReplacement {
				t.Fatalf("replacement = %v, want %v; pod-number diff: %#v", requiresNew, tc.wantReplacement, podNumberDiff)
			}
			if !tc.wantReplacement && diff != nil {
				if field := diff.Attributes["node_config.0.desired_pod_num"]; field != nil {
					t.Fatalf("unexpected pod-number drift: %#v", field)
				}
			}
		})
	}
}

func TestKubernetesNodePoolDesiredPodNumCreate(t *testing.T) {
	for _, tc := range []struct {
		name         string
		configured   *int
		wantComputed bool
	}{
		{name: "omitted delegates to cloud", wantComputed: true},
		{name: "explicit preserves caller intent", configured: intPointer(32)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := svctke.ResourceTencentCloudKubernetesNodePool()
			node := map[string]interface{}{"user_data": "ZWNobyB0ZXN0"}
			if tc.configured != nil {
				node["desired_pod_num"] = *tc.configured
			}
			config := terraform.NewResourceConfigRaw(map[string]interface{}{
				"cluster_id": "cls-example", "name": "example", "vpc_id": "vpc-example",
				"min_size": 0, "max_size": 100,
				"auto_scaling_config": []interface{}{map[string]interface{}{"instance_type": "M9.2XLARGE64"}},
				"node_config":         []interface{}{node},
			})
			diff, err := r.Diff(context.Background(), nil, config, nil)
			if err != nil {
				t.Fatal(err)
			}
			if diff == nil {
				t.Fatal("missing creation diff")
			}
			field := diff.Attributes["node_config.0.desired_pod_num"]
			if field == nil || field.NewComputed != tc.wantComputed {
				t.Fatalf("pod-number creation diff = %#v; computed want %v", field, tc.wantComputed)
			}
			if tc.configured != nil && field.New != "32" {
				t.Fatalf("explicit value not preserved: %#v", field)
			}
		})
	}
}

func intPointer(v int) *int { return &v }
