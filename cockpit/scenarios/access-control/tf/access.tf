# ---------------------------------------------------------------------------
# Access control: who may publish and subscribe to what
#
# One client profile, and an ACL profile per role. Each ACL profile denies
# publishing and subscribing by default and allows exactly the topics the
# role needs, as exceptions. The broker enforces them at the moment an app
# publishes or subscribes.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_client_profile" "ac" {
  msg_vpn_name        = var.msg_vpn
  client_profile_name = "cp-ac"

  # Guaranteed sending is allowed so a refused publish comes back on its
  # receipt; a direct publish to a forbidden topic is dropped silently.
  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = false
  allow_guaranteed_endpoint_create_enabled = false

  max_connection_count_per_client_username = 10
  max_endpoint_count_per_client_username   = 0
}

locals {
  # role => { publish = [topics], subscribe = [topics] }
  ac_roles = {
    payments  = { publish = ["ac/bank/payment/>", "ac/bank/public/>"], subscribe = ["ac/bank/>"] }
    reporting = { publish = [], subscribe = ["ac/bank/payment/>"] }
    partner   = { publish = [], subscribe = ["ac/bank/public/>"] }
  }
  ac_publish = { for pair in flatten([for role, r in local.ac_roles : [for t in r.publish : { role = role, topic = t }]]) : "${pair.role}:${pair.topic}" => pair }
  ac_subscribe = { for pair in flatten([for role, r in local.ac_roles : [for t in r.subscribe : { role = role, topic = t }]]) : "${pair.role}:${pair.topic}" => pair }
}

resource "solacebroker_msg_vpn_acl_profile" "ac" {
  for_each = local.ac_roles

  msg_vpn_name     = var.msg_vpn
  acl_profile_name = "acl-ac-${each.key}"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "disallow"
  subscribe_topic_default_action = "disallow"
}

resource "solacebroker_msg_vpn_acl_profile_publish_topic_exception" "ac" {
  for_each = local.ac_publish

  msg_vpn_name                   = var.msg_vpn
  acl_profile_name               = solacebroker_msg_vpn_acl_profile.ac[each.value.role].acl_profile_name
  publish_topic_exception_syntax = "smf"
  publish_topic_exception        = each.value.topic
}

resource "solacebroker_msg_vpn_acl_profile_subscribe_topic_exception" "ac" {
  for_each = local.ac_subscribe

  msg_vpn_name                     = var.msg_vpn
  acl_profile_name                 = solacebroker_msg_vpn_acl_profile.ac[each.value.role].acl_profile_name
  subscribe_topic_exception_syntax = "smf"
  subscribe_topic_exception        = each.value.topic
}

resource "solacebroker_msg_vpn_client_username" "ac" {
  for_each = local.ac_roles

  msg_vpn_name    = var.msg_vpn
  client_username = "svc-ac-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.ac.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.ac[each.key].acl_profile_name
}
