package tke

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/connectivity"
)

type endpointImportMeta struct {
	client *connectivity.TencentCloudClient
}

func (m endpointImportMeta) GetAPIV3Conn() *connectivity.TencentCloudClient { return m.client }

type endpointImportTransport func(*http.Request) (*http.Response, error)

func (f endpointImportTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Exercise the registered Terraform import hook, including the real SDK request.
// Import must only observe access modes; it must not enable or disable endpoints.
func TestTkeClusterEndpointImport(t *testing.T) {
	for _, tt := range []struct {
		name, response              string
		internet, intranet, wantErr bool
	}{
		{"public", `{"Response":{"ClusterExternalEndpoint":"public.example:443","PgwEndpoint":""}}`, true, false, false},
		{"private", `{"Response":{"ClusterExternalEndpoint":"","PgwEndpoint":"10.0.0.1:443"}}`, false, true, false},
		{"both", `{"Response":{"ClusterExternalEndpoint":"public.example:443","PgwEndpoint":"10.0.0.1:443"}}`, true, true, false},
		{"neither", `{"Response":{}}`, false, false, false},
		{"denied", `{"Response":{"Error":{"Code":"UnauthorizedOperation","Message":"denied"}}}`, false, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := ResourceTencentCloudTkeClusterEndpoint()
			if r.Importer == nil || r.Importer.StateContext == nil {
				t.Fatal("ClusterEndpoint must support Terraform import for Crossplane Observe-only recovery")
			}
			calls := 0
			reading := false
			previous := http.DefaultTransport
			http.DefaultTransport = endpointImportTransport(func(req *http.Request) (*http.Response, error) {
				calls++
				action := req.Header.Get("X-TC-Action")
				var body map[string]interface{}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if action != "DescribeClusters" && body["ClusterId"] != "cls-existing" {
					t.Fatalf("unexpected cluster ID: %v", body["ClusterId"])
				}
				response := tt.response
				switch action {
				case "DescribeClusterSecurity":
				case "DescribeClusters":
					if !reading {
						t.Fatal("unexpected cluster read in import hook")
					}
					response = `{"Response":{"Clusters":[{"ClusterId":"cls-existing","ClusterOs":"os","ClusterVersion":"1.34.1","ClusterDescription":"test","ClusterName":"existing","ClusterStatus":"Running","ProjectId":0,"ClusterNodeNum":1,"ClusterType":"MANAGED_CLUSTER","Property":"{}","ClusterNetworkSettings":{"VpcId":"vpc-existing","Ipvs":false}}]}}`
				case "DescribeClusterKubeconfig":
					if !reading {
						t.Fatal("unexpected credential read in import hook")
					}
					if body["IsExtranet"] == true {
						if !tt.internet {
							t.Fatal("read requested a disabled public endpoint")
						}
						response = `{"Response":{"Kubeconfig":"synthetic-public"}}`
					} else {
						if !tt.intranet {
							t.Fatal("read requested a disabled private endpoint")
						}
						response = `{"Response":{"Kubeconfig":"synthetic-private"}}`
					}
				default:
					t.Fatalf("unexpected API action during import/read: %s", action)
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(response)), Request: req}, nil
			})
			t.Cleanup(func() { http.DefaultTransport = previous })
			meta := endpointImportMeta{client: &connectivity.TencentCloudClient{
				Credential: common.NewCredential("test-id", "test-key"), Region: "ap-beijing", Protocol: "https", Domain: "tencentcloudapi.com",
			}}
			d := schema.TestResourceDataRaw(t, r.Schema, nil)
			d.SetId("cls-existing")
			states, err := r.Importer.StateContext(context.Background(), d, meta)
			if (err != nil) != tt.wantErr {
				t.Fatalf("import error = %v, wantErr %t", err, tt.wantErr)
			}
			if calls != 1 {
				t.Fatalf("API calls = %d, want exactly one read", calls)
			}
			if tt.wantErr {
				return
			}
			if len(states) != 1 || states[0] != d || d.Id() != "cls-existing" || d.Get("cluster_id") != "cls-existing" {
				t.Fatal("import did not preserve the existing cluster identity")
			}
			if d.Get("cluster_internet") != tt.internet || d.Get("cluster_intranet") != tt.intranet {
				t.Fatalf("wrong imported access modes: public=%v private=%v", d.Get("cluster_internet"), d.Get("cluster_intranet"))
			}
			reading = true
			if err := r.Read(d, meta); err != nil {
				t.Fatalf("read after import: %v", err)
			}
			wantCalls := 3 // Import + DescribeClusters + DescribeClusterSecurity.
			if tt.internet {
				wantCalls++
				if d.Get("kube_config") != "synthetic-public" {
					t.Fatal("public kubeconfig not recovered")
				}
			}
			if tt.intranet {
				wantCalls++
				if d.Get("kube_config_intranet") != "synthetic-private" {
					t.Fatal("private kubeconfig not recovered")
				}
			}
			if calls != wantCalls {
				t.Fatalf("API calls = %d, want %d", calls, wantCalls)
			}
		})
	}
}
