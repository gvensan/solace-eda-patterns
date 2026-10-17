# ---------------------------------------------------------------------------
# The reporter's queue
#
# The sensors publish direct; this queue's subscription stores a copy of
# every reading as a guaranteed message, for the reporter. The display has
# no queue: it takes the readings direct.
#
#   max_delivered_unacked_msgs_per_flow
#       The broker's brake: it sends the reporter at most this many readings
#       that are not yet acknowledged. The reporter acknowledges a reading
#       once it is written, so a slow database holds the flow at this limit
#       and the rest waits here.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "readings" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.bp.readings"

  access_type     = "exclusive"
  ingress_enabled = true
  egress_enabled  = true
  permission      = "consume"

  max_msg_spool_usage                 = 200
  max_delivered_unacked_msgs_per_flow = 100
}

resource "solacebroker_msg_vpn_queue_subscription" "readings" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.readings.queue_name
  subscription_topic = "bp/coldchain/temperature/v1/>"
}
