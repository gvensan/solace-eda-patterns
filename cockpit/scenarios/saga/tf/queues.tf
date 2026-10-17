# ---------------------------------------------------------------------------
# Saga: queues
#
# One queue per service, subscribed to exactly the events that service
# reacts to. Nobody coordinates: each service's reaction is the next step.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "orders" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.saga.orders"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "orders" {
  for_each = toset([
    "saga/shop/stock/reserved/v1/>",
    "saga/shop/payment/refunded/v1/>",
  ])

  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.orders.queue_name
  subscription_topic = each.key
}

resource "solacebroker_msg_vpn_queue" "payments" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.saga.payments"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "payments" {
  for_each = toset([
    "saga/shop/order/created/v1/>",
    "saga/shop/stock/failed/v1/>",
  ])

  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.payments.queue_name
  subscription_topic = each.key
}

resource "solacebroker_msg_vpn_queue" "inventory" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.saga.inventory"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "inventory" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.inventory.queue_name
  subscription_topic = "saga/shop/payment/taken/v1/>"
}
