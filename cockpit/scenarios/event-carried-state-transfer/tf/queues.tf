# ---------------------------------------------------------------------------
# Event-carried state transfer: queues
#
# Shipping's queue takes both orders and customer records, in the order the
# broker received them. They come from two services, so there is no shared
# publish order; but the customer service waits for the broker to confirm
# each record, so an order placed after a change is behind it on the queue.
# Invoicing's queue takes orders only.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "shipping" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.ecst.shipping"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "shipping" {
  for_each = toset([
    "ecst/crm/customer/updated/v1/>",
    "ecst/shop/order/placed/v1/>",
  ])

  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.shipping.queue_name
  subscription_topic = each.key
}

resource "solacebroker_msg_vpn_queue" "invoicing" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.ecst.invoicing"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "invoicing" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.invoicing.queue_name
  subscription_topic = "ecst/shop/order/placed/v1/>"
}
