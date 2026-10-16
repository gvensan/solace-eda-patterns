# ---------------------------------------------------------------------------
# Exclusive consumer: who may connect
#
# One client profile and one ACL profile for the scenario, and two usernames:
# the payments service that publishes postings, and the ledger. Both ledger
# instances log in as the same username. They are the same application, run
# twice for availability, and the broker tells them apart by client name.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "excl" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-excl"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "excl" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-excl"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  excl_roles = ["payments", "ledger"]
}

resource "solacebroker_msg_vpn_client_username" "excl" {
  for_each = toset(local.excl_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-excl-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.excl.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.excl.acl_profile_name
}
