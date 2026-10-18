# ---------------------------------------------------------------------------
# Intermediated governance: who may connect, and what the central team owns
#
# Every team's ACL profile denies by default. The sales team may publish
# under ig/sales/. The inventory team may subscribe to nothing until the
# central team approves a request: the approved exceptions are added by
# central.sh, the stand-in for the central team's pipeline, and are not in
# terraform. Play revokes and re-approves them, so it always starts from
# requests.json.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "ig" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-ig"

  allow_guaranteed_msg_send_enabled        = false
  allow_guaranteed_msg_receive_enabled     = false
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

resource "solacebroker_msg_vpn_acl_profile" "ig" {
  for_each = toset(["sales", "inventory"])

  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-ig-${each.key}"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "disallow"
  subscribe_topic_default_action = "disallow"
}

resource "solacebroker_msg_vpn_acl_profile_publish_topic_exception" "ig_sales" {
  msg_vpn_name                   = var.msg_vpn
  acl_profile_name               = solacebroker_msg_vpn_acl_profile.ig["sales"].acl_profile_name
  publish_topic_exception_syntax = "smf"
  publish_topic_exception        = "ig/sales/>"
}

resource "solacebroker_msg_vpn_client_username" "ig" {
  for_each = toset(["sales", "inventory"])

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-ig-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.ig.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.ig[each.key].acl_profile_name
}
