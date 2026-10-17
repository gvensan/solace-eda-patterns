# ---------------------------------------------------------------------------
# Backpressure: who may connect
#
# The sensors publish readings; the reporter reads them from a queue and the
# display takes them directly. Each has its own username.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "bp" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-bp"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "bp" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-bp"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  bp_roles = ["sensors", "reporter", "display"]
}

resource "solacebroker_msg_vpn_client_username" "bp" {
  for_each = toset(local.bp_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-bp-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.bp.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.bp.acl_profile_name
}
