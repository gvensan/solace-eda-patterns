# ---------------------------------------------------------------------------
# Event bridge, the stores' side: who may connect
#
# The store's till, in the workshop's own VPN. Head office's users are in
# hq.tf, in head office's VPN.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "br" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-br"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "br" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-br"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  br_roles = ["store"]
}

resource "solacebroker_msg_vpn_client_username" "br" {
  for_each = toset(local.br_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-br-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.br.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.br.acl_profile_name
}
