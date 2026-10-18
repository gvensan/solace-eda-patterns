# ---------------------------------------------------------------------------
# Disintermediated governance: the guardrails, and nothing else
#
# Each team owns a topic prefix, dg/teams/<team>/, and may publish and
# subscribe there without asking anyone. Any team may subscribe to what
# another team publishes under its public/ level. Each team may create its
# own queues (self-service), up to the limit in the client profile: the
# guardrail that keeps one team from filling the broker.
#
# Nothing here is per request. Adding a team is adding it to the list.
# ---------------------------------------------------------------------------

locals {
  dg_teams = ["orders", "billing"]
}

resource "solacebroker_msg_vpn_client_profile" "dg" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-dg"

  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = true

  max_connection_count_per_client_username = 10
  # The quota guardrail: one queue per team.
  max_endpoint_count_per_client_username = 1
}

resource "solacebroker_msg_vpn_acl_profile" "dg" {
  for_each = toset(local.dg_teams)

  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-dg-${each.key}"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "disallow"
  subscribe_topic_default_action = "disallow"
}

resource "solacebroker_msg_vpn_acl_profile_publish_topic_exception" "dg" {
  for_each = toset(local.dg_teams)

  msg_vpn_name                   = var.msg_vpn
  acl_profile_name               = solacebroker_msg_vpn_acl_profile.dg[each.key].acl_profile_name
  publish_topic_exception_syntax = "smf"
  publish_topic_exception        = "dg/teams/${each.key}/>"
}

resource "solacebroker_msg_vpn_acl_profile_subscribe_topic_exception" "dg_own" {
  for_each = toset(local.dg_teams)

  msg_vpn_name                     = var.msg_vpn
  acl_profile_name                 = solacebroker_msg_vpn_acl_profile.dg[each.key].acl_profile_name
  subscribe_topic_exception_syntax = "smf"
  subscribe_topic_exception        = "dg/teams/${each.key}/>"
}

resource "solacebroker_msg_vpn_acl_profile_subscribe_topic_exception" "dg_public" {
  for_each = toset(local.dg_teams)

  msg_vpn_name                     = var.msg_vpn
  acl_profile_name                 = solacebroker_msg_vpn_acl_profile.dg[each.key].acl_profile_name
  subscribe_topic_exception_syntax = "smf"
  subscribe_topic_exception        = "dg/teams/*/public/>"
}

resource "solacebroker_msg_vpn_client_username" "dg" {
  for_each = toset(local.dg_teams)

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-dg-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.dg.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.dg[each.key].acl_profile_name
}
