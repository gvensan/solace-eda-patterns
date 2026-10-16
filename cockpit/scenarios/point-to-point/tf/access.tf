# ---------------------------------------------------------------------------
# Point-to-point: who may connect
#
# The order service sends label jobs; the label printer reads them. Each has
# its own username so the broker can say which of them did what.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "p2p" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-p2p"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  # A guaranteed message that no queue takes is normally dropped without a
  # word. With this on, the broker tells the sender instead, which is what a
  # sender addressing one named queue wants to hear if the name is wrong.
  reject_msg_to_sender_on_no_subscription_match_enabled = true

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "p2p" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-p2p"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  p2p_roles = ["orders", "printer"]
}

resource "solacebroker_msg_vpn_client_username" "p2p" {
  for_each = toset(local.p2p_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-p2p-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.p2p.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.p2p.acl_profile_name
}
