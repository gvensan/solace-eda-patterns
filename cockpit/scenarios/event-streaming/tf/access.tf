# ---------------------------------------------------------------------------
# Event streaming: who may connect
#
# The bikes publish ride events; the counter reads them from a queue and the
# board takes them direct. Each has its own username.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "es" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-es"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "es" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-es"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  es_roles = ["bikes", "counter", "board"]
}

resource "solacebroker_msg_vpn_client_username" "es" {
  for_each = toset(local.es_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-es-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.es.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.es.acl_profile_name
}
