# ---------------------------------------------------------------------------
# Hierarchical topics: who may connect
#
# The stores publish sales; every team subscribes with its own wildcard
# subscription, under one username.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "ht" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-ht"

  # Direct messaging only.
  allow_guaranteed_msg_send_enabled        = false
  allow_guaranteed_msg_receive_enabled     = false
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "ht" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-ht"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  ht_roles = ["stores", "teams"]
}

resource "solacebroker_msg_vpn_client_username" "ht" {
  for_each = toset(local.ht_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-ht-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.ht.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.ht.acl_profile_name
}
