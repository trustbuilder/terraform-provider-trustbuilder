resource "trustbuilder_idhub_tenant" "test" {
  path = "/tenants"
  data = jsonencode(local.tenant_body)
}


# Import block example
import {
  to = trustbuilder_idhub_tenant.test
  id = format("/tenants,%s", local.tenant)
}
