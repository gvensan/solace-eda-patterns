# ---------------------------------------------------------------------------
# Event sourcing: who may connect
#
# The ledger publishes every transaction as an event and rebuilds its
# balances from the replay log. The statements job uses the same username.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "esrc" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-esrc"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "esrc" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-esrc"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  esrc_roles = ["ledger"]
}

resource "solacebroker_msg_vpn_client_username" "esrc" {
  for_each = toset(local.esrc_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-esrc-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.esrc.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.esrc.acl_profile_name
}
