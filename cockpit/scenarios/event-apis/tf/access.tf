# ---------------------------------------------------------------------------
# Event APIs: who may connect
#
# The shipping service publishes the shipping event API; the labels app is
# written against v1, the tracking app against v2.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "ea" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-ea"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "ea" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-ea"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  ea_roles = ["shipping", "labels", "tracking"]
}

resource "solacebroker_msg_vpn_client_username" "ea" {
  for_each = toset(local.ea_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-ea-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.ea.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.ea.acl_profile_name
}
