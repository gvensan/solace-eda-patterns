# ---------------------------------------------------------------------------
# Change data capture: queues
#
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "search" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.cdc.search"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "search" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.search.queue_name
  subscription_topic = "cdc/inventory/stock/changed/v1/>"
}
