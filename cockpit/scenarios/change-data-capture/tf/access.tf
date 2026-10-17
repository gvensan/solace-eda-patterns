# ---------------------------------------------------------------------------
# Change data capture: who may connect
#
# The connector publishes the inventory database's changes; the search
# index reads them. The inventory app itself has no username: it never
# connects to the broker.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "cdc" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-cdc"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "cdc" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-cdc"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  cdc_roles = ["connector", "search"]
}

resource "solacebroker_msg_vpn_client_username" "cdc" {
  for_each = toset(local.cdc_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-cdc-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.cdc.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.cdc.acl_profile_name
}
