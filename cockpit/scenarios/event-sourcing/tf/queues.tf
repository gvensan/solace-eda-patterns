# ---------------------------------------------------------------------------
# Event sourcing: queues
#
# The ledger's exclusive queue. Client-initiated replay is refused on a
# non-exclusive queue (docs/spikes.md, section 17), and a replay replaces
# whatever the queue holds with the logged events.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "ledger" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.esrc.ledger"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "ledger" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.ledger.queue_name
  subscription_topic = "esrc/bank/account/>"
}
