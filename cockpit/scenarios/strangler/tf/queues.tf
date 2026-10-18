# ---------------------------------------------------------------------------
# Strangler: queues
#
# Which system gets refunds is decided here, by queue subscriptions, and
# nothing else. At the start q.stg.legacy subscribes to every order event
# and q.stg.refunds to nothing. The scenario's route.sh moves the refund
# subscription between them; the next Apply puts this starting point back.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "legacy" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.stg.legacy"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "legacy" {
  for_each = toset([
    "stg/shop/order/placed/v1/>",
    "stg/shop/order/paid/v1/>",
    "stg/shop/refund/requested/v1/>",
  ])

  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.legacy.queue_name
  subscription_topic = each.key
}

resource "solacebroker_msg_vpn_queue" "refunds" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.stg.refunds"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}
