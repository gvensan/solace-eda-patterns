# ---------------------------------------------------------------------------
# Shock absorber: who may connect
#
# The on-sale page publishes purchase requests; the booking workers share the
# queue. Each has its own username.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "sa" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-sa"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "sa" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-sa"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  sa_roles = ["onsale", "booking"]
}

resource "solacebroker_msg_vpn_client_username" "sa" {
  for_each = toset(local.sa_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-sa-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.sa.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.sa.acl_profile_name
}
