package provider

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/trustbuilder/terraform-provider-trustbuilder/fakeserver"
	"github.com/trustbuilder/terraform-provider-trustbuilder/internal/apiclient"
)

var idhubTenantResourceName = "trustbuilder_idhub_tenant"

// The identifier attribute contains the tenant.
var idhubTenantsDataObjects = map[string]map[string]any{
	"1": {
		"Test_case":        "normal",
		"identifier":       "tenant_1",
		"id":               "1",
		"repo_name_prefix": "tenant_1-sxxlh",
		"Revision":         1,
		"Thing":            "potato",
		"Is_cat":           false,
		"Colors":           []string{"orange", "white"},
		"Attrs": map[string]any{
			"size":   "6 in",
			"weight": "10 oz",
		},
	},
	"2": {
		"Test_case":        "minimal",
		"identifier":       "tenant_2",
		"id":               "2",
		"repo_name_prefix": "tenant_2-frohu",
		"Thing":            "fork",
	},
	"3": {
		"Test_case":        "no Colors",
		"identifier":       "tenant_name",
		"id":               "3",
		"repo_name_prefix": "tenant_3-oyush",
		"Thing":            "paper",
		"Is_cat":           false,
		"Attrs": map[string]any{
			"height": "8.5 in",
			"width":  "11 in",
		},
	},
	"4": {
		"Test_case":        "no Attrs",
		"identifier":       "tenant_4",
		"id":               "4",
		"repo_name_prefix": "tenant_4-pzovb",
		"Thing":            "nothing",
		"Is_cat":           false,
		"Colors":           []string{"none"},
	},
	"5": {
		"Test_case":        "pet",
		"identifier":       "tenant_5",
		"id":               "5",
		"repo_name_prefix": "tenant_1-ikczz",
		"Thing":            "cat",
		"Is_cat":           true,
		"Colors":           []string{"orange", "white"},
		"Attrs": map[string]any{
			"size":   "1.5 ft",
			"weight": "15 lb",
		},
	},
}

func testAccIdhubTenantPreCheck(t *testing.T) {
	debug := false
	svr := fakeserver.NewFakeServer(19090, idhubTenantsDataObjects, true, debug, "")

	t.Cleanup(func() {
		svr.Shutdown()
	})
}

func generateIdhubTenantResource(name string, data string, params map[string]any) string {
	strData, _ := json.Marshal(data)
	config := []string{
		`path = "/api/objects"`,
		fmt.Sprintf("data = %s", strData),
	}
	for k, v := range params {
		entry := fmt.Sprintf(`%s = %v`, k, v)
		config = append(config, entry)
	}
	strConfig := ""
	for _, v := range config {
		strConfig = strConfig + v + "\n"
	}

	return fmt.Sprintf(`
		resource "%s" "%s" {
		%s
	}`, idhubTenantResourceName, name, strConfig)
}

func TestAccIdhubTenantResource_basic(t *testing.T) {
	var firstUpdatedTime string
	var initialDataMap map[string]any
	var modifiedDataMap map[string]any
	var initialDataString string
	var modifiedDataString string
	var err error

	resourceName := "api_data"
	resourceFulleName := idhubTenantResourceName + "." + resourceName
	initialDataMap = map[string]any{
		"Test_case":        "newTest",
		"identifier":       "tenant_6",
		"id":               "6",
		"repo_name_prefix": "tenant_6-tclas",
		"Thing":            "basic",
	}
	initialDataString, err = apiclient.JsonEncode(initialDataMap)
	if err != nil {
		t.Errorf("test's initial data JSON encoding error: %s", err)
	}

	modifiedDataMap = make(map[string]any)
	for k, v := range initialDataMap {
		modifiedDataMap[k] = v
	}
	modifiedDataMap["additional_field"] = "new API feature"
	modifiedDataString, err = apiclient.JsonEncode(modifiedDataMap)
	if err != nil {
		t.Errorf("test's modified data JSON encoding error: %s", err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccIdhubTenantPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.RequireAbove(tfversion.Version1_11_0),
		},
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: providerConfig + generateIdhubTenantResource(resourceName, initialDataString, nil),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectKnownValue(resourceFulleName, tfjsonpath.New("data"), knownvalue.Null()),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceFulleName, "id"),
					resource.TestCheckResourceAttrSet(resourceFulleName, "last_updated"),
					//Setup the last_updated change verification
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources[resourceFulleName]
						if !ok {
							t.Error("Custom resource check: resource not found in state")
						}
						lastUpdated := rs.Primary.Attributes["last_updated"]
						if lastUpdated == "" {
							t.Error("Custom resource check: last_updated is empty")
						}
						firstUpdatedTime = lastUpdated
						return nil
					},
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(resourceFulleName, tfjsonpath.New("id"), knownvalue.StringExact("6")),
					statecheck.ExpectKnownValue(resourceFulleName, tfjsonpath.New("tenant"), knownvalue.StringExact("tenant_6")),
					statecheck.ExpectKnownValue(resourceFulleName, tfjsonpath.New("repo_name_prefix"), knownvalue.StringExact("tenant_6-tclas")),
					statecheck.ExpectKnownValue(resourceFulleName, tfjsonpath.New("last_updated"), knownvalue.StringRegexp(regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(Z|[+-]\d{2}:\d{2})$`))),
					statecheck.ExpectKnownValue(resourceFulleName, tfjsonpath.New("data"), knownvalue.Null()),
				},
			},
			// Update and Read testing
			{
				Config: providerConfig + generateIdhubTenantResource(resourceName, modifiedDataString, nil),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectKnownValue(resourceFulleName, tfjsonpath.New("data"), knownvalue.Null()),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceFulleName, "id"),
					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet(resourceFulleName, "last_updated"),
					//Verify that the last_updated has changed
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources[resourceFulleName]
						if !ok {
							t.Error("Custom resource check: resource not found in state")
						}
						lastUpdated := rs.Primary.Attributes["last_updated"]
						if lastUpdated != firstUpdatedTime {
							t.Error("Custom resource check: expected last_updated not changed, but it did")
						}
						return nil
					},
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(resourceFulleName, tfjsonpath.New("id"), knownvalue.StringExact("6")),
					statecheck.ExpectKnownValue(resourceFulleName, tfjsonpath.New("tenant"), knownvalue.StringExact("tenant_6")),
					statecheck.ExpectKnownValue(resourceFulleName, tfjsonpath.New("repo_name_prefix"), knownvalue.StringExact("tenant_6-tclas")),
					statecheck.ExpectKnownValue(resourceFulleName, tfjsonpath.New("data"), knownvalue.Null()),
				},
			},
			// Returns API error when the API object to create already exists
			{
				Config:      providerConfig + generateIdhubTenantResource("error", `{"id":"1"}`, map[string]any{}),
				ExpectError: regexp.MustCompile(".*Create request error.*"),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccIdhubTenantResource_import(t *testing.T) {
	resourceName := "api_data"
	resourceFulleName := idhubTenantResourceName + "." + resourceName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccIdhubTenantPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Configure an existing API data
			{
				Config: providerConfig +
					generateIdhubTenantResource(resourceName, `{"Test_case":"import","identifier":"tenant_7","id":"7","repo_name_prefix":"tenant_7-uvztr","Thing":"import_block"}`, nil),
			},
			{
				ResourceName:    resourceFulleName,
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				ImportStateId:   "/api/objects,tenant_7",
			},
			{
				ResourceName:    resourceFulleName,
				ImportState:     true,
				ImportStateKind: resource.ImportCommandWithID,
				ImportStateId:   "/api/objects,tenant_7",
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}
