# ---------------------------------------------------------------------------
# Data synchronization: who may connect
#
# The old CRM and the new CRM, each publishing its own edits and applying
# the other's.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "dsync" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-dsync"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "dsync" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-dsync"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  dsync_roles = ["legacy", "new"]
}

resource "solacebroker_msg_vpn_client_username" "dsync" {
  for_each = toset(local.dsync_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-dsync-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.dsync.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.dsync.acl_profile_name
}
