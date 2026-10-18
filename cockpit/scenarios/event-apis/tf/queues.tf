# ---------------------------------------------------------------------------
# Event APIs: queues
#
# One queue per consumer, each subscribed to the version of the API its
# app was written for.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "labels" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.ea.labels"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "labels" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.labels.queue_name
  subscription_topic = "ea/shipping/parcel/dispatched/v1/>"
}

resource "solacebroker_msg_vpn_queue" "tracking" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.ea.tracking"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "tracking" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.tracking.queue_name
  subscription_topic = "ea/shipping/parcel/dispatched/v2/>"
}
