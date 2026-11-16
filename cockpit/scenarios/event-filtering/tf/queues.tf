# ---------------------------------------------------------------------------
# Event filtering: queues
#
# q.ef.berlin filters by topic: its subscription names exactly the readings
# it wants, and the broker puts nothing else on it.
#
# q.ef.alarms subscribes to every reading, because severity is not in the
# topic, and its consumer binds with a selector. Readings the selector does
# not match stay on the queue (docs/spikes.md, sections 2 and 18). Its 10 MB
# quota lasts about four hours at the normal rate (about 670 bytes a second
# left behind), and about 30 seconds in high resolution (about 375 KB a
# second), measured 2026-11-16.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "berlin" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.ef.berlin"

  access_type         = "exclusive"
  ingress_enabled     = true
  egress_enabled      = true
  permission          = "consume"
  max_msg_spool_usage = 50
  # The default, stated: a reading this queue cannot take is refused to the
  # sensors and stored on no other queue either.
  reject_msg_to_sender_on_discard_behavior = "when-queue-enabled"
  max_delivered_unacked_msgs_per_flow      = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "berlin" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.berlin.queue_name
  subscription_topic = "ef/plant/berlin/*/temp/>"
}

resource "solacebroker_msg_vpn_queue" "alarms" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.ef.alarms"

  access_type         = "exclusive"
  ingress_enabled     = true
  egress_enabled      = true
  permission          = "consume"
  max_msg_spool_usage = 10
  # The default, stated: a reading this queue cannot take is refused to the
  # sensors and stored on no other queue either.
  reject_msg_to_sender_on_discard_behavior = "when-queue-enabled"
  max_delivered_unacked_msgs_per_flow      = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "alarms" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.alarms.queue_name
  subscription_topic = "ef/plant/>"
}
