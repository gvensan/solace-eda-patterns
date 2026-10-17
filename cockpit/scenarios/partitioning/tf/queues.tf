# ---------------------------------------------------------------------------
# The partitioned queue
#
# Non-exclusive, so several consumers share the work, but split into
# partitions. Every message's partition key (here the order id) picks its
# partition, and each partition goes to exactly one consumer. So all of one
# order's updates reach the same consumer, in order, while different orders
# are handled in parallel. The broker spreads the partitions over whoever is
# bound and moves them when a consumer comes or goes.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "orders" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.part.orders"

  access_type     = "non-exclusive"
  ingress_enabled = true
  egress_enabled  = true
  permission      = "consume"

  # Six partitions for three consumers, so each owns two. A fourth consumer
  # takes one, leaving two, two, one and one: the broker keeps the counts
  # within one of each other.
  partition_count = 6

  # Short, so partitions move while the attendee is still watching: the
  # broker waits 5 seconds after a consumer comes or goes before it
  # rebalances, and gives a consumer 3 seconds to hand a partition over.
  partition_rebalance_delay            = 5
  partition_rebalance_max_handoff_time = 3

  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 5
}

resource "solacebroker_msg_vpn_queue_subscription" "orders" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.orders.queue_name
  subscription_topic = "part/shop/order/>"
}
