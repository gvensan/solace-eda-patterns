# ---------------------------------------------------------------------------
# Event-carried state transfer: who may connect
#
# The customer service (crm) owns customer records and answers lookups. The
# order service publishes orders. Shipping keeps a copy of every customer;
# invoicing asks the customer service for each order.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "ecst" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-ecst"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "ecst" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-ecst"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  ecst_roles = ["crm", "orders", "shipping", "invoicing"]
}

resource "solacebroker_msg_vpn_client_username" "ecst" {
  for_each = toset(local.ecst_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-ecst-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.ecst.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.ecst.acl_profile_name
}
