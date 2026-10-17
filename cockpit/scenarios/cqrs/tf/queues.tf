# ---------------------------------------------------------------------------
# CQRS: queues
#
# One queue per side. Commands wait on q.cqrs.commands for the write side;
# events wait on q.cqrs.view for the read side. Neither side reads the
# other's queue.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "commands" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.cqrs.commands"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "commands" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.commands.queue_name
  subscription_topic = "cqrs/shop/command/place-order/v1/>"
}

resource "solacebroker_msg_vpn_queue" "view" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.cqrs.view"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "view" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.view.queue_name
  subscription_topic = "cqrs/shop/order/placed/v1/>"
}
