# ---------------------------------------------------------------------------
# Event gateway: queues
#
# Dispatch's queue takes van positions and partner orders alike, whatever
# protocol they arrived in. q.gw.billing takes the orders for the REST
# delivery point (rdp.tf), which posts each one to billing's webhook.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "dispatch" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.gw.dispatch"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "dispatch" {
  for_each = toset([
    "gw/fleet/>",
    "gw/partner/>",
  ])

  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.dispatch.queue_name
  subscription_topic = each.key
}

resource "solacebroker_msg_vpn_queue" "billing" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.gw.billing"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "billing" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.billing.queue_name
  subscription_topic = "gw/partner/order/>"
}
