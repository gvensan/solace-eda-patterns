# ---------------------------------------------------------------------------
# Event bridge: head office's VPN
#
# A second message VPN on the same broker stands in for head office's own
# broker: separate users, topics and queues. A Standard broker allows three
# VPNs in all, the workshop's own included (docs/spikes.md, section 1).
#
# The bridge logs in to this VPN as svc-br-bridge, so this VPN's client
# profile must allow bridge connections, and it binds q.br.to-stores: the
# queue that collects what head office sends to the stores. Its subscription
# is the list of what crosses the bridge.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn" "hq" {
  msg_vpn_name              = "br-hq"
  enabled                   = true
  authentication_basic_type = "internal"
  max_msg_spool_usage       = 100
}

resource "solacebroker_msg_vpn_client_profile" "hq" {
  msg_vpn_name        = solacebroker_msg_vpn.hq.msg_vpn_name
  client_profile_name = "cp-br-hq"

  allow_bridge_connections_enabled         = true
  allow_guaranteed_msg_send_enabled        = true
  allow_guaranteed_msg_receive_enabled     = true
  allow_guaranteed_endpoint_create_enabled = false
}

resource "solacebroker_msg_vpn_acl_profile" "hq" {
  msg_vpn_name     = solacebroker_msg_vpn.hq.msg_vpn_name
  acl_profile_name = "acl-br-hq"

  client_connect_default_action  = "allow"
  publish_topic_default_action   = "allow"
  subscribe_topic_default_action = "allow"
}

resource "solacebroker_msg_vpn_client_username" "hq" {
  for_each = toset(["pricing", "bridge"])

  msg_vpn_name    = solacebroker_msg_vpn.hq.msg_vpn_name
  client_username = "svc-br-${each.key}"
  password        = var.client_password
  enabled         = true

  client_profile_name = solacebroker_msg_vpn_client_profile.hq.client_profile_name
  acl_profile_name    = solacebroker_msg_vpn_acl_profile.hq.acl_profile_name
}

resource "solacebroker_msg_vpn_queue" "to_stores" {
  msg_vpn_name = solacebroker_msg_vpn.hq.msg_vpn_name
  queue_name   = "q.br.to-stores"

  access_type         = "exclusive"
  ingress_enabled     = true
  egress_enabled      = true
  permission          = "consume"
  max_msg_spool_usage = 50
}

resource "solacebroker_msg_vpn_queue_subscription" "to_stores" {
  msg_vpn_name       = solacebroker_msg_vpn.hq.msg_vpn_name
  queue_name         = solacebroker_msg_vpn_queue.to_stores.queue_name
  subscription_topic = "br/hq/price/>"
}
