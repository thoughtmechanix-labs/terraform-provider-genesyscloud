package outbound_campaign

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"terraform-provider-genesyscloud/genesyscloud/outbound"
	"testing"

	gcloud "terraform-provider-genesyscloud/genesyscloud"
	obCallableTimeset "terraform-provider-genesyscloud/genesyscloud/outbound_callabletimeset"
	obContactList "terraform-provider-genesyscloud/genesyscloud/outbound_contact_list"
	edgeSite "terraform-provider-genesyscloud/genesyscloud/telephony_providers_edges_site"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/mypurecloud/platform-client-sdk-go/v119/platformclientv2"
)

// Add a special generator DEVENGAGE-1646.  Basically, the API makes it look like you need a full phone_columns field here.  However, the API ignores the type because the devs reused the phone_columns object.  However,
// we still need to pass in a phone column block to get the column name.
func generatePhoneColumnNoTypeBlock(columnName string) string {

	return fmt.Sprintf(`
	phone_columns {
		column_name = "%s"
	}
`, columnName)
}

func TestAccResourceOutboundCampaignBasic(t *testing.T) {
	t.Parallel()

	var (
		resourceId            = "campaign1"
		name                  = "Test Campaign " + uuid.NewString()
		dialingMode           = "agentless"
		callerName            = "Test Name"
		callerAddress         = "+353371111111"
		contactListResourceId = "contact_list"
		dncListResourceId     = "dnc"
		wrapupCodeResourceId  = "wrapupcode"
		locationResourceId    = "location"
		clfResourceId         = "clf"
		carResourceId         = "car"
		ruleSetResourceId     = "rule_set"
		siteId                = "site"
		callableTimeSetId     = "time_set"
		outboundFlowFilePath  = "../../examples/resources/genesyscloud_flow/outboundcall_flow_example.yaml"
		flowName              = "test flow " + uuid.NewString()

		contactSortFieldName = "zipcode"
		contactSortDirection = "ASC"
		contactSortNumeric   = gcloud.FalseValue

		nameUpdated          = "Test Campaign " + uuid.NewString()
		callerNameUpdated    = "Test Name 2"
		callerAddressUpdated = "+353371112111"
	)

	emergencyNumber := "+13178793428"
	if err := edgeSite.DeleteLocationWithNumber(emergencyNumber, sdkConfig); err != nil {
		t.Skipf("failed to delete location with number %s: %v", emergencyNumber, err)
	}

	referencedResources := fmt.Sprintf(`
		data "genesyscloud_auth_division_home" "home" {}
		`) + obContactList.GenerateOutboundContactList(
		contactListResourceId,
		"contact list "+uuid.NewString(),
		gcloud.NullValue,
		strconv.Quote("Cell"),
		[]string{strconv.Quote("Cell")},
		[]string{strconv.Quote("Cell"), strconv.Quote("Home"), strconv.Quote("zipcode")},
		gcloud.FalseValue,
		gcloud.NullValue,
		gcloud.NullValue,
		obContactList.GeneratePhoneColumnsBlock(
			"Cell",
			"cell",
			strconv.Quote("Cell"),
		),
		obContactList.GeneratePhoneColumnsBlock(
			"Home",
			"home",
			strconv.Quote("Home"),
		),
		obContactList.GeneratePhoneColumnsDataTypeSpecBlock(
			strconv.Quote("zipcode"), // columnName
			strconv.Quote("TEXT"),    // columnDataType
			gcloud.NullValue,         // min
			gcloud.NullValue,         // max
			"10",                     // maxLength
		),
	) + outbound.GenerateOutboundDncListBasic(
		dncListResourceId,
		"dnc list "+uuid.NewString(),
	) + gcloud.GenerateRoutingWrapupcodeResource(
		wrapupCodeResourceId,
		"tf wrapup code"+uuid.NewString(),
	) + gcloud.GenerateFlowResource(
		"flow",
		outboundFlowFilePath,
		"",
		false,
		gcloud.GenerateSubstitutionsMap(map[string]string{
			"flow_name":          flowName,
			"home_division_name": "${data.genesyscloud_auth_division_home.home.name}",
			"contact_list_name":  "${genesyscloud_outbound_contact_list." + contactListResourceId + ".name}",
			"wrapup_code_name":   "${genesyscloud_routing_wrapupcode." + wrapupCodeResourceId + ".name}",
		}),
	) + outbound.GenerateOutboundCallAnalysisResponseSetResource(
		carResourceId,
		"tf car "+uuid.NewString(),
		gcloud.FalseValue,
		outbound.GenerateCarsResponsesBlock(
			outbound.GenerateCarsResponse(
				"callable_person",
				"transfer_flow",
				flowName,
				"${genesyscloud_flow.flow.id}",
			),
		),
	) + outbound.GenerateOutboundContactListFilter(
		clfResourceId,
		"tf clf "+uuid.NewString(),
		"genesyscloud_outbound_contact_list."+contactListResourceId+".id",
		"",
		outbound.GenerateOutboundContactListFilterClause(
			"",
			outbound.GenerateOutboundContactListFilterPredicates(
				"Cell",
				"alphabetic",
				"EQUALS",
				"+12345123456",
				"",
				"",
			),
		),
	) + gcloud.GenerateLocationResource(
		locationResourceId,
		"tf location "+uuid.NewString(),
		"HQ1",
		[]string{},
		gcloud.GenerateLocationEmergencyNum(
			"+13178793428",
			gcloud.NullValue,
		),
		gcloud.GenerateLocationAddress(
			"7601 Interactive Way",
			"Indianapolis",
			"IN",
			"US",
			"46278",
		),
	) + edgeSite.GenerateSiteResourceWithCustomAttrs(
		siteId,
		"tf site "+uuid.NewString(),
		"test description",
		"genesyscloud_location."+locationResourceId+".id",
		"Cloud",
		false,
		"[\"us-east-1\"]",
		gcloud.NullValue,
		gcloud.NullValue,
	) + fmt.Sprintf(`
		resource "genesyscloud_outbound_ruleset" "%s" {
			name            = "%s"
			contact_list_id = genesyscloud_outbound_contact_list.%s.id
		}
		`, ruleSetResourceId, "tf ruleset "+uuid.NewString(), contactListResourceId,
	) + obCallableTimeset.GenerateOutboundCallabletimeset(
		callableTimeSetId,
		"tf timeset "+uuid.NewString(),
		obCallableTimeset.GenerateCallableTimesBlock(
			"Africa/Abidjan",
			obCallableTimeset.GenerateTimeSlotsBlock("07:00:00", "18:00:00", "3"),
		))

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { gcloud.TestAccPreCheck(t) },
		ProviderFactories: gcloud.GetProviderFactories(providerResources, providerDataSources),
		Steps: []resource.TestStep{
			{
				Config: referencedResources + generateOutboundCampaign(
					resourceId,
					name,
					dialingMode,
					callerName,
					callerAddress,
					"genesyscloud_outbound_contact_list."+contactListResourceId+".id",
					gcloud.NullValue, // campaign_status
					gcloud.NullValue, // division id
					gcloud.NullValue, // script id
					gcloud.NullValue, // queue id
					"genesyscloud_telephony_providers_edges_site."+siteId+".id",
					"1",
					"genesyscloud_outbound_callabletimeset."+callableTimeSetId+".id",
					"genesyscloud_outbound_callanalysisresponseset."+carResourceId+".id",
					"1",
					gcloud.NullValue,
					"0",
					gcloud.FalseValue,
					"40",
					"4",
					[]string{"genesyscloud_outbound_dnclist." + dncListResourceId + ".id"},
					[]string{"genesyscloud_outbound_ruleset." + ruleSetResourceId + ".id"},
					[]string{"genesyscloud_outbound_contactlistfilter." + clfResourceId + ".id"},
					generatePhoneColumnNoTypeBlock("Cell"),
					outbound.GenerateOutboundMessagingCampaignContactSort(
						contactSortFieldName,
						contactSortDirection,
						contactSortNumeric,
					),
					generateDynamicContactQueueingSettingsBlock(gcloud.TrueValue),
				),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "name", name),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "dialing_mode", dialingMode),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "caller_name", callerName),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "caller_address", callerAddress),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "outbound_line_count", "1"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "campaign_status", "off"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "abandon_rate", "1"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "preview_time_out_seconds", "0"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "always_running", gcloud.FalseValue),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "no_answer_timeout", "40"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "priority", "4"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "phone_columns.0.column_name", "Cell"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "contact_sorts.0.field_name", contactSortFieldName),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "contact_sorts.0.direction", contactSortDirection),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "contact_sorts.0.numeric", contactSortNumeric),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "dynamic_contact_queueing_settings.0.sort", gcloud.TrueValue),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "contact_list_id",
						"genesyscloud_outbound_contact_list."+contactListResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "callable_time_set_id",
						"genesyscloud_outbound_callabletimeset."+callableTimeSetId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "contact_list_filter_ids.0",
						"genesyscloud_outbound_contactlistfilter."+clfResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "dnc_list_ids.0",
						"genesyscloud_outbound_dnclist."+dncListResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "rule_set_ids.0",
						"genesyscloud_outbound_ruleset."+ruleSetResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "site_id",
						"genesyscloud_telephony_providers_edges_site."+siteId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "call_analysis_response_set_id",
						"genesyscloud_outbound_callanalysisresponseset."+carResourceId, "id"),
					gcloud.TestDefaultHomeDivision("genesyscloud_outbound_campaign."+resourceId),
				),
			},
			{
				// Update DCQSettings (ForceNew)
				Config: referencedResources + generateOutboundCampaign(
					resourceId,
					name,
					dialingMode,
					callerName,
					callerAddress,
					"genesyscloud_outbound_contact_list."+contactListResourceId+".id",
					gcloud.NullValue, // campaign_status
					gcloud.NullValue, // division id
					gcloud.NullValue, // script id
					gcloud.NullValue, // queue id
					"genesyscloud_telephony_providers_edges_site."+siteId+".id",
					"1",
					"genesyscloud_outbound_callabletimeset."+callableTimeSetId+".id",
					"genesyscloud_outbound_callanalysisresponseset."+carResourceId+".id",
					"1",
					gcloud.NullValue,
					"0",
					gcloud.FalseValue,
					"40",
					"4",
					[]string{"genesyscloud_outbound_dnclist." + dncListResourceId + ".id"},
					[]string{"genesyscloud_outbound_ruleset." + ruleSetResourceId + ".id"},
					[]string{"genesyscloud_outbound_contactlistfilter." + clfResourceId + ".id"},
					generatePhoneColumnNoTypeBlock("Cell"),
					outbound.GenerateOutboundMessagingCampaignContactSort(
						contactSortFieldName,
						contactSortDirection,
						contactSortNumeric,
					),
					generateDynamicContactQueueingSettingsBlock(gcloud.FalseValue),
				),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "name", name),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "dialing_mode", dialingMode),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "caller_name", callerName),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "caller_address", callerAddress),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "outbound_line_count", "1"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "campaign_status", "off"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "abandon_rate", "1"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "preview_time_out_seconds", "0"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "always_running", gcloud.FalseValue),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "no_answer_timeout", "40"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "priority", "4"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "phone_columns.0.column_name", "Cell"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "contact_sorts.0.field_name", contactSortFieldName),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "contact_sorts.0.direction", contactSortDirection),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "contact_sorts.0.numeric", contactSortNumeric),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "dynamic_contact_queueing_settings.0.sort", gcloud.FalseValue),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "contact_list_id",
						"genesyscloud_outbound_contact_list."+contactListResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "callable_time_set_id",
						"genesyscloud_outbound_callabletimeset."+callableTimeSetId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "contact_list_filter_ids.0",
						"genesyscloud_outbound_contactlistfilter."+clfResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "dnc_list_ids.0",
						"genesyscloud_outbound_dnclist."+dncListResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "rule_set_ids.0",
						"genesyscloud_outbound_ruleset."+ruleSetResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "site_id",
						"genesyscloud_telephony_providers_edges_site."+siteId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "call_analysis_response_set_id",
						"genesyscloud_outbound_callanalysisresponseset."+carResourceId, "id"),
					gcloud.TestDefaultHomeDivision("genesyscloud_outbound_campaign."+resourceId),
				),
			},
			{
				// Update
				Config: referencedResources + generateOutboundCampaign(
					resourceId,
					nameUpdated,
					dialingMode,
					callerNameUpdated,
					callerAddressUpdated,
					"genesyscloud_outbound_contact_list."+contactListResourceId+".id",
					gcloud.NullValue, // campaign_status
					gcloud.NullValue, // division id
					gcloud.NullValue, // script id
					gcloud.NullValue, // queue id
					"genesyscloud_telephony_providers_edges_site."+siteId+".id",
					"2",
					"genesyscloud_outbound_callabletimeset."+callableTimeSetId+".id",
					"genesyscloud_outbound_callanalysisresponseset."+carResourceId+".id",
					"2",
					gcloud.NullValue,
					"1",
					gcloud.TrueValue,
					"30",
					"3",
					[]string{"genesyscloud_outbound_dnclist." + dncListResourceId + ".id"},
					[]string{"genesyscloud_outbound_ruleset." + ruleSetResourceId + ".id"},
					[]string{"genesyscloud_outbound_contactlistfilter." + clfResourceId + ".id"},
					generatePhoneColumnNoTypeBlock("Cell"),
					outbound.GenerateOutboundMessagingCampaignContactSort(
						contactSortFieldName,
						contactSortDirection,
						contactSortNumeric,
					),
					generateDynamicContactQueueingSettingsBlock(gcloud.FalseValue),
				),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "name", nameUpdated),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "dialing_mode", dialingMode),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "caller_name", callerNameUpdated),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "caller_address", callerAddressUpdated),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "outbound_line_count", "2"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "abandon_rate", "2"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "preview_time_out_seconds", "1"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "always_running", gcloud.TrueValue),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "no_answer_timeout", "30"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "priority", "3"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "phone_columns.0.column_name", "Cell"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "contact_sorts.0.field_name", contactSortFieldName),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "contact_sorts.0.direction", contactSortDirection),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "contact_sorts.0.numeric", contactSortNumeric),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "dynamic_contact_queueing_settings.0.sort", "false"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "contact_list_id",
						"genesyscloud_outbound_contact_list."+contactListResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "callable_time_set_id",
						"genesyscloud_outbound_callabletimeset."+callableTimeSetId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "contact_list_filter_ids.0",
						"genesyscloud_outbound_contactlistfilter."+clfResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "dnc_list_ids.0",
						"genesyscloud_outbound_dnclist."+dncListResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "rule_set_ids.0",
						"genesyscloud_outbound_ruleset."+ruleSetResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "site_id",
						"genesyscloud_telephony_providers_edges_site."+siteId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "call_analysis_response_set_id",
						"genesyscloud_outbound_callanalysisresponseset."+carResourceId, "id"),
					gcloud.TestDefaultHomeDivision("genesyscloud_outbound_campaign."+resourceId),
				),
			},
			{
				// Import/Read
				ResourceName:            "genesyscloud_outbound_campaign." + resourceId,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"queue_id"},
			},
		},
		CheckDestroy: testVerifyOutboundCampaignDestroyed,
	})
}

func TestAccResourceOutboundCampaignCampaignStatus(t *testing.T) {
	t.Parallel()
	var (
		resourceId            = "campaign2"
		name                  = "Test Campaign " + uuid.NewString()
		contactListResourceId = "contact_list"
		carResourceId         = "car"
		siteId                = "site"
		wrapupCodeResourceId  = "wrapupcode"
		outboundFlowFilePath  = "../../examples/resources/genesyscloud_flow/outboundcall_flow_example.yaml"
		flowName              = "test flow " + uuid.NewString()
		flowResourceId        = "flow"
		wrapupcodeResourceId  = "wrapupcode"
		locationResourceId    = "location"
	)

	emergencyNumber := "+13178793429"
	if err := edgeSite.DeleteLocationWithNumber(emergencyNumber, sdkConfig); err != nil {
		t.Skipf("failed to delete location with number %s: %v", emergencyNumber, err)
	}

	referencedResources := obContactList.GenerateOutboundContactList(
		contactListResourceId,
		"contact list "+uuid.NewString(),
		gcloud.NullValue,
		strconv.Quote("Cell"),
		[]string{strconv.Quote("Cell")},
		[]string{strconv.Quote("Cell"), strconv.Quote("Home"), strconv.Quote("zipcode")},
		gcloud.FalseValue,
		gcloud.NullValue,
		gcloud.NullValue,
		obContactList.GeneratePhoneColumnsBlock(
			"Cell",
			"cell",
			strconv.Quote("Cell"),
		),
		obContactList.GeneratePhoneColumnsBlock(
			"Home",
			"home",
			strconv.Quote("Home"),
		),
	) + gcloud.GenerateRoutingWrapupcodeResource(
		wrapupcodeResourceId,
		"tf wrapup code"+uuid.NewString(),
	) + gcloud.GenerateFlowResource(
		flowResourceId,
		outboundFlowFilePath,
		"",
		false,
		gcloud.GenerateSubstitutionsMap(map[string]string{
			"flow_name":          flowName,
			"home_division_name": "${data.genesyscloud_auth_division_home.home.name}",
			"contact_list_name":  "${genesyscloud_outbound_contact_list." + contactListResourceId + ".name}",
			"wrapup_code_name":   "${genesyscloud_routing_wrapupcode." + wrapupCodeResourceId + ".name}",
		}),
	) + outbound.GenerateOutboundCallAnalysisResponseSetResource(
		carResourceId,
		"tf car "+uuid.NewString(),
		gcloud.FalseValue,
		outbound.GenerateCarsResponsesBlock(
			outbound.GenerateCarsResponse(
				"callable_person",
				"transfer_flow",
				flowName,
				"${genesyscloud_flow.flow.id}",
			),
		),
	) + gcloud.GenerateLocationResource(
		locationResourceId,
		"tf location "+uuid.NewString(),
		"HQ1",
		[]string{},
		gcloud.GenerateLocationEmergencyNum(
			"+13178793429",
			gcloud.NullValue,
		),
		gcloud.GenerateLocationAddress(
			"7601 Interactive Way",
			"Indianapolis",
			"IN",
			"US",
			"46278",
		),
	) + edgeSite.GenerateSiteResourceWithCustomAttrs(
		siteId,
		"tf site "+uuid.NewString(),
		"test description",
		"genesyscloud_location."+locationResourceId+".id",
		"Cloud",
		false,
		"[\"us-east-1\"]",
		gcloud.NullValue,
		gcloud.NullValue,
	) + fmt.Sprintf("\ndata \"genesyscloud_auth_division_home\" \"home\" {}\n")

	// Test campaign_status can be turned on in a second run after first run's initial creation in off state, and then back off again
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { gcloud.TestAccPreCheck(t) },
		ProviderFactories: gcloud.GetProviderFactories(providerResources, providerDataSources),
		Steps: []resource.TestStep{
			{
				Config: referencedResources + fmt.Sprintf(`
					resource "genesyscloud_outbound_campaign" "%s" {
						name                          = "%s"
						dialing_mode                  = "agentless"
						caller_name                   = "Test Name"
						caller_address                = "+353371111111"
						outbound_line_count           = 2
						campaign_status               = "off"
						contact_list_id               = genesyscloud_outbound_contact_list.%s.id
						site_id                       = genesyscloud_telephony_providers_edges_site.%s.id
						call_analysis_response_set_id = genesyscloud_outbound_callanalysisresponseset.%s.id
						phone_columns {
							column_name = "Cell"
						}
					}
					`, resourceId, name, contactListResourceId, siteId, carResourceId),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "name", name),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "campaign_status", "off"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "contact_list_id",
						"genesyscloud_outbound_contact_list."+contactListResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "site_id",
						"genesyscloud_telephony_providers_edges_site."+siteId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "call_analysis_response_set_id",
						"genesyscloud_outbound_callanalysisresponseset."+carResourceId, "id"),
					// Add contacts to the contact list (because we have access to the state and can pull out the contactlist ID to pass to the API)
					addContactsToContactList,
				),
			},
			{
				Config: referencedResources + fmt.Sprintf(`
					resource "genesyscloud_outbound_campaign" "%s" {
						name                          = "%s"
						dialing_mode                  = "agentless"
						caller_name                   = "Test Name"
						caller_address                = "+353371111111"
						outbound_line_count           = 2
						campaign_status               = "on"
						contact_list_id               = genesyscloud_outbound_contact_list.%s.id
						site_id                       = genesyscloud_telephony_providers_edges_site.%s.id
						call_analysis_response_set_id = genesyscloud_outbound_callanalysisresponseset.%s.id
						phone_columns {
							column_name = "Cell"
						}
					}
					`, resourceId, name, contactListResourceId, siteId, carResourceId),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "name", name),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "contact_list_id",
						"genesyscloud_outbound_contact_list."+contactListResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "site_id",
						"genesyscloud_telephony_providers_edges_site."+siteId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "call_analysis_response_set_id",
						"genesyscloud_outbound_callanalysisresponseset."+carResourceId, "id"),
					gcloud.VerifyAttributeInArrayOfPotentialValues("genesyscloud_outbound_campaign."+resourceId, "campaign_status", []string{"on", "complete"}),
				),
			},
			{
				Config: referencedResources + fmt.Sprintf(`
				resource "genesyscloud_outbound_campaign" "%s" {
					name                          = "%s"
					dialing_mode                  = "agentless"
					caller_name                   = "Test Name"
					caller_address                = "+353371111111"
					outbound_line_count           = 2
					campaign_status               = "off"
					contact_list_id               = genesyscloud_outbound_contact_list.%s.id
					site_id                       = genesyscloud_telephony_providers_edges_site.%s.id
					call_analysis_response_set_id = genesyscloud_outbound_callanalysisresponseset.%s.id
					phone_columns {
						column_name = "Cell"
					}
				}
				`, resourceId, name, contactListResourceId, siteId, carResourceId),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "name", name),
					gcloud.VerifyAttributeInArrayOfPotentialValues("genesyscloud_outbound_campaign."+resourceId, "campaign_status", []string{"off", "complete"}),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "contact_list_id",
						"genesyscloud_outbound_contact_list."+contactListResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "site_id",
						"genesyscloud_telephony_providers_edges_site."+siteId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "call_analysis_response_set_id",
						"genesyscloud_outbound_callanalysisresponseset."+carResourceId, "id"),
				),
			},
			{
				// Import/Read
				ResourceName:            "genesyscloud_outbound_campaign." + resourceId,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"campaign_status"},
			},
		},
		CheckDestroy: testVerifyOutboundCampaignDestroyed,
	})
}

func TestAccResourceOutboundCampaignStatusOn(t *testing.T) {
	t.Parallel()
	var (
		resourceId            = "campaign3"
		name                  = "Test Campaign " + uuid.NewString()
		contactListResourceId = "contact_list"
		carResourceId         = "car"
		siteId                = "site"
		outboundFlowFilePath  = "../../examples/resources/genesyscloud_flow/outboundcall_flow_example.yaml"
		flowName              = "test flow " + uuid.NewString()
		flowResourceId        = "flow"
		wrapupcodeResourceId  = "wrapupcode"
		locationResourceId    = "location"
	)

	emergencyNumber := "+13178793430"
	if err := edgeSite.DeleteLocationWithNumber(emergencyNumber, sdkConfig); err != nil {
		t.Skipf("failed to delete location with number %s: %v", emergencyNumber, err)
	}

	// Test campaign_status can be turned on at time of creation as well
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { gcloud.TestAccPreCheck(t) },
		ProviderFactories: gcloud.GetProviderFactories(providerResources, providerDataSources),
		Steps: []resource.TestStep{
			// Create resources for outbound campaign
			{
				Config: fmt.Sprintf(`
data "genesyscloud_auth_division_home" "home" {}
`) + GenerateReferencedResourcesForOutboundCampaignTests(
					contactListResourceId,
					"",
					"",
					carResourceId,
					outboundFlowFilePath,
					flowResourceId,
					flowName,
					"",
					siteId,
					emergencyNumber,
					"",
					"",
					"${data.genesyscloud_auth_division_home.home.name}",
					locationResourceId,
					wrapupcodeResourceId,
				),
				// Add contacts to the contact list (because we have access to the state and can pull out the contactlist ID to pass to the API)
				Check: addContactsToContactList,
			},
			// Now, we create the outbound campaign and it should stay running because it has contacts to call. We leave it running to test
			// the destroy command takes care of turning it off before deleting.
			{
				Config: fmt.Sprintf(`
data "genesyscloud_auth_division_home" "home" {}
`) + GenerateOutboundCampaignBasic(
					resourceId,
					name,
					contactListResourceId,
					siteId,
					emergencyNumber,
					carResourceId,
					strconv.Quote("on"),
					outboundFlowFilePath,
					flowResourceId,
					flowName,
					"${data.genesyscloud_auth_division_home.home.name}",
					locationResourceId,
					wrapupcodeResourceId,
				),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "name", name),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "contact_list_id",
						"genesyscloud_outbound_contact_list."+contactListResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "site_id",
						"genesyscloud_telephony_providers_edges_site."+siteId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "call_analysis_response_set_id",
						"genesyscloud_outbound_callanalysisresponseset."+carResourceId, "id"),
					gcloud.VerifyAttributeInArrayOfPotentialValues("genesyscloud_outbound_campaign."+resourceId, "campaign_status", []string{"on", "complete"}),
				),
			},
			// Don't turn campaign back off to ensure campaign can be destroyed properly by turning it off within the destroy handler
			{
				// Import/Read
				ResourceName:            "genesyscloud_outbound_campaign." + resourceId,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"campaign_status"},
			},
		},
		CheckDestroy: testVerifyOutboundCampaignDestroyed,
	})
}

func TestAccResourceOutboundCampaignWithScriptId(t *testing.T) {
	t.Parallel()
	var (
		resourceId                = "campaign4"
		name                      = "Test Campaign " + uuid.NewString()
		dialingMode               = "preview"
		callerName                = "Test Name 123"
		callerAddress             = "+353371111111"
		contactListResourceId     = "contact_list"
		queueResourceId           = "queue"
		dncListResourceId         = "dnc_list"
		carResourceId             = "car"
		clfResourceId             = "clf"
		ruleSetResourceId         = "rule_set"
		callableTimeSetResourceId = "time_set"

		contactSortFieldName = "zipcode"
		contactSortDirection = "ASC"
		contactSortNumeric   = gcloud.FalseValue
	)

	scriptId, err := getPublishedScriptId()
	if err != nil || scriptId == "" {
		t.Skip("Skipping as a published script ID is needed to run this test")
	}

	referencedResources := GenerateReferencedResourcesForOutboundCampaignTests(
		contactListResourceId,
		"",
		queueResourceId,
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
	)

	referencedResourcesUpdate := GenerateReferencedResourcesForOutboundCampaignTests(
		contactListResourceId,
		dncListResourceId,
		queueResourceId,
		carResourceId,
		"",
		"",
		"",
		clfResourceId,
		"",
		"",
		ruleSetResourceId,
		callableTimeSetResourceId,
		"",
		"",
		"",
	)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { gcloud.TestAccPreCheck(t) },
		ProviderFactories: gcloud.GetProviderFactories(providerResources, providerDataSources),
		Steps: []resource.TestStep{
			{
				Config: referencedResources +
					generateOutboundCampaign(
						resourceId,
						name,
						dialingMode,
						callerName,
						callerAddress,
						"genesyscloud_outbound_contact_list."+contactListResourceId+".id",
						gcloud.NullValue,
						gcloud.NullValue,
						strconv.Quote(scriptId),
						"genesyscloud_routing_queue."+queueResourceId+".id",
						gcloud.NullValue,
						gcloud.NullValue,
						gcloud.NullValue,
						gcloud.NullValue,
						gcloud.NullValue,
						gcloud.NullValue,
						gcloud.NullValue,
						gcloud.NullValue,
						gcloud.NullValue,
						gcloud.NullValue,
						[]string{},
						[]string{},
						[]string{},
						generatePhoneColumnNoTypeBlock("Cell"),
					),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "name", name),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "dialing_mode", dialingMode),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "caller_name", callerName),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "caller_address", callerAddress),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "phone_columns.0.column_name", "Cell"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "script_id", scriptId),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "contact_list_id",
						"genesyscloud_outbound_contact_list."+contactListResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "queue_id",
						"genesyscloud_routing_queue."+queueResourceId, "id"),
				),
			},
			{
				// Update
				Config: referencedResourcesUpdate +
					generateOutboundCampaign(
						resourceId,
						name,
						dialingMode,
						callerName,
						callerAddress,
						"genesyscloud_outbound_contact_list."+contactListResourceId+".id",
						gcloud.NullValue,
						gcloud.NullValue,
						strconv.Quote(scriptId),
						"genesyscloud_routing_queue."+queueResourceId+".id",
						gcloud.NullValue,
						"1",
						"genesyscloud_outbound_callabletimeset."+callableTimeSetResourceId+".id",
						"genesyscloud_outbound_callanalysisresponseset."+carResourceId+".id",
						"1",
						gcloud.FalseValue,
						"1",
						gcloud.FalseValue,
						"3",
						"2",
						[]string{"genesyscloud_outbound_dnclist." + dncListResourceId + ".id"},
						[]string{"genesyscloud_outbound_ruleset." + ruleSetResourceId + ".id"},
						[]string{"genesyscloud_outbound_contactlistfilter." + clfResourceId + ".id"},
						generatePhoneColumnNoTypeBlock("Cell"),
						outbound.GenerateOutboundMessagingCampaignContactSort(
							contactSortFieldName,
							contactSortDirection,
							contactSortNumeric,
						),
					),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "name", name),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "dialing_mode", dialingMode),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "caller_name", callerName),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "caller_address", callerAddress),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "phone_columns.0.column_name", "Cell"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "abandon_rate", "1"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "outbound_line_count", "1"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "skip_preview_disabled", gcloud.FalseValue),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "preview_time_out_seconds", "1"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "always_running", gcloud.FalseValue),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "contact_sorts.0.field_name", contactSortFieldName),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "contact_sorts.0.direction", contactSortDirection),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "contact_sorts.0.numeric", contactSortNumeric),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "no_answer_timeout", "3"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "priority", "2"),
					resource.TestCheckResourceAttr("genesyscloud_outbound_campaign."+resourceId, "script_id", scriptId),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "contact_list_filter_ids.0",
						"genesyscloud_outbound_contactlistfilter."+clfResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "dnc_list_ids.0",
						"genesyscloud_outbound_dnclist."+dncListResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "contact_list_id",
						"genesyscloud_outbound_contact_list."+contactListResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "queue_id",
						"genesyscloud_routing_queue."+queueResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "call_analysis_response_set_id",
						"genesyscloud_outbound_callanalysisresponseset."+carResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "rule_set_ids.0",
						"genesyscloud_outbound_ruleset."+ruleSetResourceId, "id"),
					resource.TestCheckResourceAttrPair("genesyscloud_outbound_campaign."+resourceId, "callable_time_set_id",
						"genesyscloud_outbound_callabletimeset."+callableTimeSetResourceId, "id"),
					gcloud.TestDefaultHomeDivision("genesyscloud_outbound_campaign."+resourceId),
				),
			},
			{
				// Import/Read
				ResourceName:      "genesyscloud_outbound_campaign." + resourceId,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
		CheckDestroy: testVerifyOutboundCampaignDestroyed,
	})
}

func addContactsToContactList(state *terraform.State) error {
	outboundAPI := platformclientv2.NewOutboundApi()
	contactListResource := state.RootModule().Resources["genesyscloud_outbound_contact_list.contact_list"]
	if contactListResource == nil {
		return fmt.Errorf("genesyscloud_outbound_contact_list.contact_list contactListResource not found in state")
	}

	contactList, _, err := outboundAPI.GetOutboundContactlist(contactListResource.Primary.ID, false, false)
	if err != nil {
		return fmt.Errorf("genesyscloud_outbound_contact_list (%s) not available", contactListResource.Primary.ID)
	}
	contactsJSON := `[{
			"data": {
			  "FirstName": "Asa",
			  "LastName": "Acosta",
			  "Cell": "3335551234",
			  "Home": "3335552345",
			  "zipcode": "23849"
			},
			"callable": true,
			"phoneNumberStatus": {}
		  },
		  {
			"data": {
			  "FirstName": "Leonidas",
			  "LastName": "Acosta",
			  "Cell": "4445551234",
			  "Home": "4445552345",
			  "zipcode": "34567"
			},
			"callable": true,
			"phoneNumberStatus": {}
		  },
		  {
			"data": {
			  "FirstName": "Nolan",
			  "LastName": "Adams",
			  "Cell": "6665551234",
			  "Home": "6665552345",
			  "zipcode": "56789"
			},
			"callable": true,
			"phoneNumberStatus": {}
		  }]`
	var contacts []platformclientv2.Writabledialercontact
	err = json.Unmarshal([]byte(contactsJSON), &contacts)
	if err != nil {
		return fmt.Errorf("could not unmarshall JSON contacts to add to contact list")
	}
	_, _, err = outboundAPI.PostOutboundContactlistContacts(*contactList.Id, contacts, false, false, false)
	if err != nil {
		return fmt.Errorf("could not post contacts to contact list")
	}
	return nil
}

func generateOutboundCampaign(
	resourceId string,
	name string,
	dialingMode string, // required
	callerName string, // required
	callerAddress string, // required
	contactListId string, // required
	campaignStatus string,
	divisionId string,
	scriptId string,
	queueId string,
	siteId string,
	abandonRate string,
	callableTimeSetId string,
	callAnalysisResponseSetId string,
	outboundLineCount string,
	skipPreviewDisabled string,
	previewTimeOutSeconds string,
	alwaysRunning string,
	noAnswerTimeout string,
	priority string,
	dncListIds []string,
	ruleSetIds []string,
	contactListFilterIds []string,
	nestedBlocks ...string,
) string {
	return fmt.Sprintf(`
resource "genesyscloud_outbound_campaign" "%s" {
	name                          = "%s"
	dialing_mode                  = "%s"
	caller_name                   = "%s"
	caller_address                = "%s"
	contact_list_id               = %s
	campaign_status               = %s
	division_id                   = %s
	script_id                     = %s
	queue_id                      = %s
	site_id 					  = %s
	abandon_rate                  = %s
	callable_time_set_id          = %s
	call_analysis_response_set_id = %s
	outbound_line_count           = %s
	skip_preview_disabled         = %s
	preview_time_out_seconds      = %s
	always_running                = %s
	no_answer_timeout             = %s
	priority                      = %s
	dnc_list_ids                  = [%s]
	rule_set_ids 			      = [%s]
	contact_list_filter_ids       = [%s]
	%s
}
`, resourceId, name, dialingMode, callerName, callerAddress, contactListId, campaignStatus, divisionId, scriptId, queueId, siteId, abandonRate, callableTimeSetId,
		callAnalysisResponseSetId, outboundLineCount, skipPreviewDisabled, previewTimeOutSeconds, alwaysRunning, noAnswerTimeout,
		priority, strings.Join(dncListIds, ", "), strings.Join(ruleSetIds, ", "), strings.Join(contactListFilterIds, ", "),
		strings.Join(nestedBlocks, "\n"))
}

func generateDynamicContactQueueingSettingsBlock(sort string) string {
	return fmt.Sprintf(`
	dynamic_contact_queueing_settings {
		sort = %s
	}
	`, sort)
}

func getPublishedScriptId() (string, error) {
	api := platformclientv2.NewScriptsApiWithConfig(sdkConfig)
	// Get the published scripts.
	data, _, err := api.GetScriptsPublished(50, 1, "", "", "", "", "", "")
	if err != nil {
		return "", err
	}
	script := (*data.Entities)[0]
	return *script.Id, nil
}

func testVerifyOutboundCampaignDestroyed(state *terraform.State) error {
	outboundAPI := platformclientv2.NewOutboundApi()
	for _, rs := range state.RootModule().Resources {
		if rs.Type != "genesyscloud_outbound_campaign" {
			continue
		}
		campaign, resp, err := outboundAPI.GetOutboundCampaign(rs.Primary.ID)
		if campaign != nil {
			return fmt.Errorf("campaign (%s) still exists", rs.Primary.ID)
		} else if gcloud.IsStatus404(resp) {
			// campaign not found as expected
			continue
		} else {
			// Unexpected error
			return fmt.Errorf("unexpected error: %s", err)
		}
	}
	// Success. All campaigns destroyed
	return nil
}
