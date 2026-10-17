# ---------------------------------------------------------------------------
# The queue of purchases
#
# Exclusive: one points service reads it, in order. The settings that matter
# for guaranteed delivery:
#
#   reject_msg_to_sender_on_discard_behavior
#       What the publisher hears when the queue cannot take a message.
#       "always": it is refused, and the tills keep it for retry. The default,
#       "when-queue-enabled", stays silent while the queue's ingress is off:
#       the tills would be told the purchase was stored, and it would be
#       dropped (docs/spikes.md, section 12).
#   max_delivered_unacked_msgs_per_flow
#       How many purchases the points service may hold unacknowledged. Kept
#       small, so few come back after a crash.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "awards" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.gd.awards"

  access_type     = "exclusive"
  ingress_enabled = true
  egress_enabled  = true
  permission      = "consume"

  max_msg_spool_usage                      = 50
  max_delivered_unacked_msgs_per_flow      = 5
  reject_msg_to_sender_on_discard_behavior = "always"
}

resource "solacebroker_msg_vpn_queue_subscription" "awards" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.awards.queue_name
  subscription_topic = "gd/loyalty/points/earned/v1/>"
}
