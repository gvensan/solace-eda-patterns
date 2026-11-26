# ---------------------------------------------------------------------------
# The label queue
#
# It subscribes to the one topic the order service publishes label jobs to.
# The sender knows the topic, not the queue. (A sender can also address the
# queue by name, through its network topic #P2P/QUE/q.p2p.labels.)
#
# Exclusive, because there is one printer. With several readers sharing it,
# this becomes the Competing Consumers pattern instead.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "labels" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.p2p.labels"

  access_type     = "exclusive"
  ingress_enabled = true
  egress_enabled  = true
  permission      = "consume"

  # 1 MB, about 40 label jobs of 25 KB. Small on purpose: with the printer
  # away for half a minute the queue fills, and the broker refuses further
  # jobs back to the sender (reject_msg_to_sender_on_discard_behavior keeps
  # its default, when-queue-enabled).
  max_msg_spool_usage = 1

  max_delivered_unacked_msgs_per_flow = 5
}

# One topic, no wildcard: this queue is the one place label jobs go.
resource "solacebroker_msg_vpn_queue_subscription" "labels" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.labels.queue_name
  subscription_topic = "p2p/label/requested"
}
