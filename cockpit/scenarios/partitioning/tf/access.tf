# ---------------------------------------------------------------------------
# Partitioning: who may connect
#
# The order service publishes status updates; the fulfilment consumers read
# them. Both fulfilment consumers log in as the same username and are told
# apart by client name.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "part" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-part"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "part" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-part"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  part_roles = ["orders", "fulfilment"]
}

resource "solacebroker_msg_vpn_client_username" "part" {
  for_each = toset(local.part_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-part-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.part.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.part.acl_profile_name
}
