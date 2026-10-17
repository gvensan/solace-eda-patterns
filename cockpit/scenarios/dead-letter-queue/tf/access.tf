# ---------------------------------------------------------------------------
# Dead message queue: who may connect
#
# The shop places orders, the intake checks them, and operations watches the
# dead message queue. Each has its own username.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "dlq" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-dlq"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "dlq" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-dlq"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  dlq_roles = ["shop", "intake", "ops"]
}

resource "solacebroker_msg_vpn_client_username" "dlq" {
  for_each = toset(local.dlq_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-dlq-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.dlq.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.dlq.acl_profile_name
}
