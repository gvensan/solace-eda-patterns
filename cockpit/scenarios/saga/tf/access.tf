# ---------------------------------------------------------------------------
# Saga: who may connect
#
# Three services, each with its own data and its own queue.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "saga" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-saga"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "saga" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-saga"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  saga_roles = ["orders", "payments", "inventory"]
}

resource "solacebroker_msg_vpn_client_username" "saga" {
  for_each = toset(local.saga_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-saga-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.saga.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.saga.acl_profile_name
}
