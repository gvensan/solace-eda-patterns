# ---------------------------------------------------------------------------
# The absorber, and where stale requests go
#
# q.sa.purchases takes the on-sale page's bursts at the speed they arrive
# and the booking workers drain it at theirs. Two limits decide what it can
# absorb:
#
#   max_msg_spool_usage   its size, in MB. A burst that does not fit is
#                         refused back to the publisher (Spool Over Quota).
#                         Sized from docs/spikes.md, section 16, to hold a
#                         normal burst of 1,500 with room to spare and not a
#                         burst of 6,000.
#   respect_ttl_enabled   a request sent with a time to live (a seat hold)
#                         that is still waiting when it lapses is moved to
#                         the dead message queue, and counted.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "purchases" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.sa.purchases"

  access_type     = "non-exclusive"
  ingress_enabled = true
  egress_enabled  = true
  permission      = "consume"

  max_msg_spool_usage                 = 1
  max_delivered_unacked_msgs_per_flow = 20
  respect_ttl_enabled                 = true
  dead_msg_queue                      = solacebroker_msg_vpn_queue.dmq.queue_name
}

resource "solacebroker_msg_vpn_queue_subscription" "purchases" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.purchases.queue_name
  subscription_topic = "sa/tickets/purchase/requested/v1/>"
}

# Nothing publishes here: requests arrive only when q.sa.purchases gives up
# on them because their time to live lapsed while they waited.
resource "solacebroker_msg_vpn_queue" "dmq" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.sa.purchases.dmq"

  access_type     = "exclusive"
  ingress_enabled = true
  egress_enabled  = true
  permission      = "consume"

  max_msg_spool_usage = 10
}
