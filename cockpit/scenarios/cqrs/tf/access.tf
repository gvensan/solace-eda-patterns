# ---------------------------------------------------------------------------
# CQRS: who may connect
#
# Checkout sends commands, the order service (write) handles them and
# publishes events, the order summary service (view) builds its view from the
# events and answers queries from the account page.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "cqrs" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-cqrs"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "cqrs" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-cqrs"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  cqrs_roles = ["checkout", "write", "view", "account"]
}

resource "solacebroker_msg_vpn_client_username" "cqrs" {
  for_each = toset(local.cqrs_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-cqrs-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.cqrs.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.cqrs.acl_profile_name
}
