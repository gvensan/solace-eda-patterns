# ---------------------------------------------------------------------------
# Guaranteed delivery: who may connect
#
# The tills publish purchases and the points service reads them. Each has its
# own username.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "gd" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-gd"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "gd" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-gd"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  gd_roles = ["tills", "points"]
}

resource "solacebroker_msg_vpn_client_username" "gd" {
  for_each = toset(local.gd_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-gd-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.gd.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.gd.acl_profile_name
}
