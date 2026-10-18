# ---------------------------------------------------------------------------
# Data synchronization: queues
#
# Each CRM's queue subscribes to the other CRM's changes only, by the side
# level in the topic: dsync/crm/customer/changed/v1/<side>/<customer>. A CRM
# never receives its own edits back from the broker.
# ---------------------------------------------------------------------------

resource "solacebroker_msg_vpn_queue" "legacy" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.dsync.legacy"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "legacy" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.legacy.queue_name
  subscription_topic = "dsync/crm/customer/changed/v1/new/>"
}

resource "solacebroker_msg_vpn_queue" "new" {
  msg_vpn_name = var.msg_vpn
  queue_name   = "q.dsync.new"

  access_type                         = "exclusive"
  ingress_enabled                     = true
  egress_enabled                      = true
  permission                          = "consume"
  max_msg_spool_usage                 = 50
  max_delivered_unacked_msgs_per_flow = 10
}

resource "solacebroker_msg_vpn_queue_subscription" "new" {
  msg_vpn_name       = var.msg_vpn
  queue_name         = solacebroker_msg_vpn_queue.new.queue_name
  subscription_topic = "dsync/crm/customer/changed/v1/legacy/>"
}
