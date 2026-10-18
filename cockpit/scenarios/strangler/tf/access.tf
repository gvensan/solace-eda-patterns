# ---------------------------------------------------------------------------
# Strangler: who may connect
#
# The shop publishes order events; the old order system and the new refund
# service each read their own queue.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "stg" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-stg"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "stg" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-stg"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  stg_roles = ["shop", "legacy", "refunds"]
}

resource "solacebroker_msg_vpn_client_username" "stg" {
  for_each = toset(local.stg_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-stg-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.stg.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.stg.acl_profile_name
}
