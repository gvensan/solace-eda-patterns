# ---------------------------------------------------------------------------
# Event catalog: who may connect
#
# Two publishing teams (orders, payments), the catalog checker, and finance,
# which consumes payments as the catalog describes them.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "ec" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-ec"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "ec" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-ec"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  ec_roles = ["orders", "payments", "checker", "finance"]
}

resource "solacebroker_msg_vpn_client_username" "ec" {
  for_each = toset(local.ec_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-ec-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.ec.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.ec.acl_profile_name
}
