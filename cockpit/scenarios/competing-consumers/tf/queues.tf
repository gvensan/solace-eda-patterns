# ---------------------------------------------------------------------------
# The shared queue
#
# access_type = "non-exclusive" is the whole pattern. Every worker that binds
# takes a share of the messages; each message goes to exactly one worker.
# Compare the exclusive consumer, where only one bound consumer receives.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "emails" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.cc.emails"

  access_type     = "non-exclusive"
  ingress_enabled = true
  egress_enabled  = true
  permission      = "consume"

  max_msg_spool_usage = 50

  # Five in flight per worker. This is what a worker that crashes hands back,
  # and what a worker that hangs sits on while the others carry on.
  max_delivered_unacked_msgs_per_flow = 5
}

resource "solacebroker_msg_vpn_queue_subscription" "emails" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.emails.queue_name
  subscription_topic = "cc/shop/order/confirmed/v1/>"
}
