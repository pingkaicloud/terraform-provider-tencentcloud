package vpc_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	tcacctest "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/acctest"
	svcvpc "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/services/vpc"
)

func TestTencentCloudVpcPeerConnectManagerStateSchema(t *testing.T) {
	state, ok := svcvpc.ResourceTencentCloudVpcPeerConnectManager().Schema["state"]
	if !ok {
		t.Fatal("state schema is not defined")
	}
	if state.Type != schema.TypeString {
		t.Fatalf("state type = %v, want string", state.Type)
	}
	if !state.Computed {
		t.Fatal("state must be computed")
	}
}

func TestAccTencentCloudVpcPeerConnectManagerResource_basic(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			tcacctest.AccPreCheck(t)
		},
		Providers: tcacctest.AccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccVpcPeerConnectManager,
				Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttrSet("tencentcloud_vpc_peer_connect_manager.peer_connect_manager", "id"),
					resource.TestCheckResourceAttrSet("tencentcloud_vpc_peer_connect_manager.peer_connect_manager", "source_vpc_id"),
					resource.TestCheckResourceAttr("tencentcloud_vpc_peer_connect_manager.peer_connect_manager", "peering_connection_name", "example-iac"),
					resource.TestCheckResourceAttrSet("tencentcloud_vpc_peer_connect_manager.peer_connect_manager", "destination_vpc_id"),
					resource.TestCheckResourceAttr("tencentcloud_vpc_peer_connect_manager.peer_connect_manager", "destination_uin", "100022975249"),
					resource.TestCheckResourceAttr("tencentcloud_vpc_peer_connect_manager.peer_connect_manager", "destination_region", "ap-guangzhou"),
					resource.TestCheckResourceAttrSet("tencentcloud_vpc_peer_connect_manager.peer_connect_manager", "state"),
				),
			},
			{
				Config: testAccVpcPeerConnectManagerUpdate,
				Check: resource.ComposeTestCheckFunc(resource.TestCheckResourceAttrSet("tencentcloud_vpc_peer_connect_manager.peer_connect_manager", "id"),
					resource.TestCheckResourceAttrSet("tencentcloud_vpc_peer_connect_manager.peer_connect_manager", "source_vpc_id"),
					resource.TestCheckResourceAttr("tencentcloud_vpc_peer_connect_manager.peer_connect_manager", "peering_connection_name", "example-iac-update"),
					resource.TestCheckResourceAttrSet("tencentcloud_vpc_peer_connect_manager.peer_connect_manager", "destination_vpc_id"),
					resource.TestCheckResourceAttr("tencentcloud_vpc_peer_connect_manager.peer_connect_manager", "destination_uin", "100022975249"),
					resource.TestCheckResourceAttr("tencentcloud_vpc_peer_connect_manager.peer_connect_manager", "destination_region", "ap-guangzhou"),
					resource.TestCheckResourceAttrSet("tencentcloud_vpc_peer_connect_manager.peer_connect_manager", "state"),
				),
			},
			{
				ResourceName:            "tencentcloud_vpc_peer_connect_manager.peer_connect_manager",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"destination_vpc_id"},
			},
		},
	})
}

const testAccVpcPeerConnectManager = `

resource "tencentcloud_vpc" "vpc" {
  name       = "tf-example-pcx"
  cidr_block = "10.0.0.0/16"
}
resource "tencentcloud_vpc" "des_vpc" {
  name       = "tf-example-pcx-des"
  cidr_block = "172.16.0.0/16"
}
resource "tencentcloud_vpc_peer_connect_manager" "peer_connect_manager" {
  source_vpc_id = tencentcloud_vpc.vpc.id
  peering_connection_name = "example-iac"
  destination_vpc_id = tencentcloud_vpc.des_vpc.id
  destination_uin = 100022975249
  destination_region = "ap-guangzhou"
}

`
const testAccVpcPeerConnectManagerUpdate = `

resource "tencentcloud_vpc" "vpc" {
  name       = "tf-example-pcx"
  cidr_block = "10.0.0.0/16"
}
resource "tencentcloud_vpc" "des_vpc" {
  name       = "tf-example-pcx-des"
  cidr_block = "172.16.0.0/16"
}
resource "tencentcloud_vpc_peer_connect_manager" "peer_connect_manager" {
  source_vpc_id = tencentcloud_vpc.vpc.id
  peering_connection_name = "example-iac-update"
  destination_vpc_id = tencentcloud_vpc.des_vpc.id
  destination_uin = 100022975249
  destination_region = "ap-guangzhou"
}

`
