package tke

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/connectivity"
)

func TestNodePoolConfigurationFailurePreservesState(t *testing.T) {
	for _, tt := range []struct {
		name       string
		failAction string
		keep       *bool
		update     bool
	}{
		{"create fails configuring pool", "ModifyNodePoolInstanceTypes", common.BoolPtr(false), false},
		{"create fails configuring AS", "ModifyLaunchConfigurationAttributes", common.BoolPtr(false), false},
		{"explicit keep", "ModifyLaunchConfigurationAttributes", common.BoolPtr(true), false},
		{"default keep", "ModifyLaunchConfigurationAttributes", nil, false},
		{"existing pool update fails", "ModifyLaunchConfigurationAttributes", common.BoolPtr(false), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			wantKeep := tt.keep == nil || *tt.keep
			deleted := false
			oldTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = oldTransport })
			http.DefaultTransport = tkeTestTransport(func(req *http.Request) (*http.Response, error) {
				action := req.Header.Get("X-TC-Action")
				response := `{"Response":{}}`
				switch action {
				case tt.failAction:
					response = `{"Response":{"Error":{"Code":"InvalidParameterValue.CvmConfigurationError","Message":"injected configuration failure"}}}`
				case "CreateClusterNodePool":
					response = `{"Response":{"NodePoolId":"np-test"}}`
				case "DescribeClusters":
					response = `{"Response":{"Clusters":[{"ClusterId":"cls-test","ClusterName":"test","ClusterType":"MANAGED_CLUSTER","ClusterStatus":"Running","ClusterOs":"test","ClusterVersion":"1.34.1","ClusterDescription":"","Property":"{}","ProjectId":0,"ClusterNodeNum":0,"ClusterNetworkSettings":{"VpcId":"vpc-test","Ipvs":false}}]}}`
				case "DescribeClusterNodePoolDetail":
					if !deleted {
						response = `{"Response":{"NodePool":{"NodePoolId":"np-test","LifeState":"normal","LaunchConfigurationId":"asc-test","AutoscalingGroupId":"asg-test","AutoscalingGroupStatus":"enabled","MinNodesNum":0,"MaxNodesNum":10,"DesiredNodesNum":0,"NodeCountSummary":{"AutoscalingAdded":{"Total":0},"ManuallyAdded":{"Total":0}}}}}`
					}
				case "ModifyNodePoolInstanceTypes", "DescribeLaunchConfigurations", "DescribeAutoScalingGroups":
				case "DeleteClusterNodePool":
					var body struct {
						ClusterID    string   `json:"ClusterId"`
						NodePoolIDs  []string `json:"NodePoolIds"`
						KeepInstance *bool
					}
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						return nil, err
					}
					if body.ClusterID != "cls-test" || len(body.NodePoolIDs) != 1 || body.NodePoolIDs[0] != "np-test" ||
						body.KeepInstance == nil || *body.KeepInstance != wantKeep {
						t.Errorf("unexpected delete request: %+v, want cls-test#np-test with KeepInstance=%t", body, wantKeep)
					}
					deleted = true
				default:
					return nil, fmt.Errorf("unexpected action %s", action)
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(response)), Request: req}, nil
			})

			meta := tkeTestMeta{client: &connectivity.TencentCloudClient{
				Credential: common.NewCredential("test-id", "test-key"), Region: "ap-shanghai", Protocol: "https", Domain: "example.invalid",
			}}
			launchConfig := map[string]interface{}{
				"instance_type": "S9.LARGE8", "system_disk_size": 50, "key_ids": []interface{}{"skey-test"},
			}
			config := map[string]interface{}{
				"cluster_id": "cls-test", "name": "test", "vpc_id": "vpc-test", "min_size": 0, "max_size": 10,
				"auto_scaling_config": []interface{}{launchConfig},
			}
			if tt.keep != nil {
				config["delete_keep_instance"] = *tt.keep
			}
			r := ResourceTencentCloudKubernetesNodePool()
			var prior *terraform.InstanceState
			if tt.update {
				d := schema.TestResourceDataRaw(t, r.Schema, config)
				d.SetId("cls-test#np-test")
				prior = d.State()
				launchConfig["system_disk_size"] = 80
			}
			ctx := context.Background()
			diff, err := r.Diff(ctx, prior, terraform.NewResourceConfigRaw(config), meta)
			if err != nil || diff == nil || (tt.update && diff.RequiresNew()) {
				t.Fatalf("unexpected diff: %v, error: %v", diff, err)
			}
			state, diags := r.Apply(ctx, prior, diff, meta)
			if !diags.HasError() || !strings.Contains(fmt.Sprint(diags), "injected configuration failure") {
				t.Fatalf("expected %s to fail: %v", tt.failAction, diags)
			}
			if state == nil || state.ID != "cls-test#np-test" || state.Attributes["delete_keep_instance"] != strconv.FormatBool(wantKeep) {
				t.Fatalf("failed apply lost pool ID or delete policy: %#v", state)
			}
			if tt.update && state.Attributes["auto_scaling_config.0.system_disk_size"] != "50" {
				t.Fatal("failed update recorded the unapplied disk size")
			}
			state, diags = r.RefreshWithoutUpgrade(ctx, state, meta)
			if diags.HasError() || state == nil {
				t.Fatalf("refresh after failure: %v", diags)
			}
			if _, diags = r.Apply(ctx, state, &terraform.InstanceDiff{Destroy: true}, meta); diags.HasError() || !deleted {
				t.Fatalf("destroy after failure: %v", diags)
			}
		})
	}
}
