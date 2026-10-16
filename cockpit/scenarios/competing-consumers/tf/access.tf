# ---------------------------------------------------------------------------
# Competing consumers: who may connect
#
# Checkout publishes; the workers consume. Every worker logs in as the same
# username, because they are the same application run several times; the
# broker tells them apart by client name.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "cc" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-cc"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "cc" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-cc"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  cc_roles = ["checkout", "worker"]
}

resource "solacebroker_msg_vpn_client_username" "cc" {
  for_each = toset(local.cc_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-cc-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.cc.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.cc.acl_profile_name
}
