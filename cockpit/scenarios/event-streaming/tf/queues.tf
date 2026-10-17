# ---------------------------------------------------------------------------
# The stream
#
# Every ride event is published as a guaranteed message, and this queue's
# subscription keeps them in the order they were published. The counter
# reads it: the queue is its position in the stream. Exclusive, because the
# stream has one order and one counter keeps it.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "rides" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.es.rides"

  access_type     = "exclusive"
  ingress_enabled = true
  egress_enabled  = true
  permission      = "consume"

  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "rides" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.rides.queue_name
  subscription_topic = "es/bikes/ride/>"
}
