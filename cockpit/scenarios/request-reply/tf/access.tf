# ---------------------------------------------------------------------------
# Request-reply: who may connect
#
# The tills ask; the pricing service answers. Each has its own username.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "rr" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-rr"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "rr" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-rr"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  rr_roles = ["tills", "pricing"]
}

resource "solacebroker_msg_vpn_client_username" "rr" {
  for_each = toset(local.rr_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-rr-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.rr.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.rr.acl_profile_name
}
