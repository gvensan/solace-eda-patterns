# ---------------------------------------------------------------------------
# Event bridge, the stores' side: queues
#
# The store's queue, in the stores' VPN. It subscribes to everything head
# office publishes, br/hq/>; what actually arrives is what the bridge
# carries (bridge.tf).
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "store" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.br.store"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "store" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.store.queue_name
  subscription_topic = "br/hq/>"
}
