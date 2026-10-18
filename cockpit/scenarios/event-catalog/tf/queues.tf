# ---------------------------------------------------------------------------
# Event catalog: queues
#
# Finance's queue, subscribed to the payment events the catalog documents.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "finance" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.ec.finance"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "finance" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.finance.queue_name
  subscription_topic = "ec/shop/payment/taken/v1/>"
}
