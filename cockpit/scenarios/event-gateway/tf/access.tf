# ---------------------------------------------------------------------------
# Event gateway: who may connect
#
# The vans connect over MQTT and the partner over HTTP, each with its own
# username; dispatch uses the Solace API. Billing has no username: the broker
# posts to it.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "gw" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-gw"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "gw" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-gw"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

locals {
  gw_roles = ["vans", "partner", "dispatch"]
}

resource "solacebroker_msg_vpn_client_username" "gw" {
  for_each = toset(local.gw_roles)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-gw-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.gw.client_profile_name
  acl_profile_name = (each.key == "partner"
    ? solacebroker_msg_vpn_acl_profile.gw_partner.acl_profile_name
    : solacebroker_msg_vpn_acl_profile.gw.acl_profile_name)
}

# The partner is outside the company: it may post orders under gw/partner/
# and nothing else. The broker checks every post, whatever the protocol.
resource "solacebroker_msg_vpn_acl_profile" "gw_partner" {
  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-gw-partner"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "disallow"
  subscribe_topic_default_action = "disallow"
}

resource "solacebroker_msg_vpn_acl_profile_publish_topic_exception" "gw_partner" {
  msg_vpn_name                   = var.msg_vpn
  acl_profile_name               = solacebroker_msg_vpn_acl_profile.gw_partner.acl_profile_name
  publish_topic_exception_syntax = "smf"
  publish_topic_exception        = "gw/partner/>"
}
