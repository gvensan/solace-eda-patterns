# ---------------------------------------------------------------------------
# Consumer groups: who may connect
#
# The catalogue publishes price changes; each group's members read their own
# group's queue. Each group's members share one username.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "cg" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-cg"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "cg" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-cg"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  cg_roles = ["catalog", "search", "alerts"]
}

resource "solacebroker_msg_vpn_client_username" "cg" {
  for_each = toset(local.cg_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-cg-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.cg.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.cg.acl_profile_name
}
