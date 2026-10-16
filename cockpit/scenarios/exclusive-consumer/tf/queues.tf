# ---------------------------------------------------------------------------
# The exclusive queue
#
# access_type = "exclusive" is the whole pattern. Any number of consumers may
# bind, but the broker delivers to one of them at a time: the first to bind.
# The others are standbys. When the active consumer unbinds or disconnects,
# the broker makes the next one active and gives it everything the first had
# not acknowledged, so order is kept across the handover.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "postings" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.excl.postings"

  access_type     = "exclusive"
  ingress_enabled = true
  egress_enabled  = true
  permission      = "consume"

  max_msg_spool_usage = 50

  # Five in flight to the active consumer. This is how much a stalled
  # consumer can sit on, and how much comes back marked redelivered when the
  # active one goes away. Small, so both are easy to count in the log.
  max_delivered_unacked_msgs_per_flow = 5
}

# Every posting, whatever the account. The topic prefix is unique to this
# scenario, so no other scenario's queue picks these up.
resource "solacebroker_msg_vpn_queue_subscription" "postings" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.postings.queue_name
  subscription_topic = "excl/ledger/posting/v1/>"
}
