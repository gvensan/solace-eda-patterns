# ---------------------------------------------------------------------------
# Publish-subscribe: who may connect
#
# The shop publishes orders; shipping, analytics and email each subscribe.
# Each has its own username.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "ps" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-ps"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "ps" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-ps"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  ps_roles = ["shop", "shipping", "analytics", "email"]
}

resource "solacebroker_msg_vpn_client_username" "ps" {
  for_each = toset(local.ps_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-ps-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.ps.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.ps.acl_profile_name
}
