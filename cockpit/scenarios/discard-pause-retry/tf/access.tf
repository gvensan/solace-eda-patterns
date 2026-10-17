# ---------------------------------------------------------------------------
# Discard, pause and retry: who may connect
#
# The shop sends payments, the processor charges them at the bank by
# request-reply, and the bank answers.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "dpr" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-dpr"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "dpr" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-dpr"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  dpr_roles = ["shop", "bank", "processor"]
}

resource "solacebroker_msg_vpn_client_username" "dpr" {
  for_each = toset(local.dpr_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-dpr-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.dpr.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.dpr.acl_profile_name
}
