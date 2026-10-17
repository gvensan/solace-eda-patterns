# ---------------------------------------------------------------------------
# Event filtering: who may connect
#
# The sensors publish readings; Berlin temperatures and the alarm desk each
# read their own queue.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "ef" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-ef"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "ef" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-ef"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  ef_roles = ["sensors", "berlin", "alarms"]
}

resource "solacebroker_msg_vpn_client_username" "ef" {
  for_each = toset(local.ef_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-ef-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.ef.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.ef.acl_profile_name
}
