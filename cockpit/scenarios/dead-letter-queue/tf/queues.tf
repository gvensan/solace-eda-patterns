# ---------------------------------------------------------------------------
# The order queue and its dead message queue
#
# The settings that decide what happens to an order that can never be
# processed:
#
#   max_redelivery_count          how many redeliveries before giving up
#                                 (2: three attempts in all)
#   dead_msg_queue                where a message goes when the queue gives up
#   respect_dmq_eligible_enabled  whether the publisher's DMQ-eligible flag
#                                 counts
#
# Written for broker 10.26.0. There, with respect_dmq_eligible_enabled false
# (its default), every message the queue removes goes to the DMQ whatever its
# flag says. On 10.8.1 the attribute does not exist (remove the line to apply
# there) and the flag alone decides. docs/spikes.md, section 6, has the
# measurements.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "orders" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.dlq.orders"

  access_type     = "exclusive"
  ingress_enabled = true
  egress_enabled  = true
  permission      = "consume"

  max_msg_spool_usage = 50

  max_redelivery_count         = 2
  dead_msg_queue               = solacebroker_msg_vpn_queue.dmq.queue_name
  respect_dmq_eligible_enabled = false
}

resource "solacebroker_msg_vpn_queue_subscription" "orders" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.orders.queue_name
  subscription_topic = "dlq/shop/order/placed/v1/>"
}

# Nothing publishes here. Messages arrive only when q.dlq.orders gives up
# on them, and stay until someone deals with them.
resource "solacebroker_msg_vpn_queue" "dmq" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.dlq.orders.dmq"

  access_type     = "exclusive"
  ingress_enabled = true
  egress_enabled  = true
  permission      = "consume"

  max_msg_spool_usage = 50
}
