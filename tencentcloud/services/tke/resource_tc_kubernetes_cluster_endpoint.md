Provide a resource to create a kubernetes cluster endpoint. This resource allows you to create an empty cluster first without any workers. Only all attached node depends create complete, cluster endpoint will finally be enabled.

~> **NOTE:** Recommend using `depends_on` to make sure endpoint create after node pools or workers does.

~> **NOTE:** Please do not use this resource and resource `tencentcloud_kubernetes_cluster` to operate cluster public network/intranet access at the same time.

Example Usage

Open intranet access for kubernetes cluster

```hcl
resource "tencentcloud_kubernetes_cluster_endpoint" "example" {
  cluster_id                 = "cls-fdy7hm1q"
  cluster_intranet           = true
  cluster_intranet_subnet_id = "subnet-7nl0sswi"
  cluster_intranet_domain    = "intranet_demo.com"
}
```

Open internet access for kubernetes cluster

```hcl
resource "tencentcloud_kubernetes_cluster_endpoint" "example" {
  cluster_id                      = "cls-fdy7hm1q"
  cluster_internet                = true
  cluster_internet_security_group = "sg-e6a8xxib"
  cluster_internet_domain         = "internet_demo.com"
  extensive_parameters = jsonencode({
    "AddressIPVersion" : "IPV4",
    "InternetAccessible" : {
      "InternetChargeType" : "TRAFFIC_POSTPAID_BY_HOUR",
      "InternetMaxBandwidthOut" : 10
    }
  })
}
```

Import

Existing cluster endpoint settings can be imported using the cluster ID:

```shell
terraform import tencentcloud_kubernetes_cluster_endpoint.example cls-existing
```

Import only reads the existing endpoint configuration. It initializes the public
and private access flags so the following refresh can retrieve kubeconfig for
enabled endpoints; it does not open, close or recreate either endpoint.

Keep the configured domains, security groups and subnet consistent with the
existing cluster, and review the plan before applying changes. For Crossplane
recovery, use Observe-only management policies until all references and desired
settings are verified. Import alone does not restore unrelated cluster, node
pool or network resource associations.
